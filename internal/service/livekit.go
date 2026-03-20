package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"github.com/AugustLigh/GoMino/pkg/config"
	"github.com/livekit/protocol/auth"
	"gorm.io/gorm"
)

type LiveKitService struct {
	db     *gorm.DB
	config config.LiveKitConfig
}

func NewLiveKitService(db *gorm.DB, cfg config.LiveKitConfig) *LiveKitService {
	return &LiveKitService{
		db:     db,
		config: cfg,
	}
}

// StartRoom - начать live комнату (только для CoHost/Host, или любой участник в DM)
func (s *LiveKitService) StartRoom(threadID, hostUID, title string, roomType chat.LiveRoomType) (*chat.LiveRoom, string, error) {
	// Проверить права пользователя в чате
	var member chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", threadID, hostUID).First(&member).Error; err != nil {
		return nil, "", errors.New("user is not a member of this thread")
	}

	// В DM (type=1) любой участник может начать звонок
	var thread chat.Thread
	if err := s.db.Where("thread_id = ?", threadID).First(&thread).Error; err != nil {
		return nil, "", errors.New("thread not found")
	}

	isDM := thread.Type != nil && *thread.Type == 1
	if !isDM && member.Role != chat.MemberRoleHost && member.Role != chat.MemberRoleCoHost {
		return nil, "", errors.New("only host or co-host can start live room")
	}

	// Проверить нет ли уже активной комнаты в этом чате
	var existingRooms []chat.LiveRoom
	if err := s.db.Where("thread_id = ? AND status = ?", threadID, chat.LiveRoomStatusActive).Limit(1).Find(&existingRooms).Error; err != nil {
		return nil, "", err
	}
	if len(existingRooms) > 0 {
		return nil, "", errors.New("there is already an active live room in this thread")
	}

	// Создать комнату
	roomID := fmt.Sprintf("room_%s_%d", threadID, time.Now().UnixNano())

	room := &chat.LiveRoom{
		RoomID:           roomID,
		ThreadID:         threadID,
		Title:            title,
		Type:             roomType,
		Status:           chat.LiveRoomStatusActive,
		HostUID:          hostUID,
		ParticipantCount: 1,
		MaxParticipants:  100,
	}

	if err := s.db.Create(room).Error; err != nil {
		return nil, "", fmt.Errorf("failed to create room: %w", err)
	}

	// Загрузить host данные
	s.db.Preload("Host").First(room, room.ID)

	// Сгенерировать токен для хоста
	token, err := s.generateToken(roomID, hostUID, true)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return room, token, nil
}

// JoinRoom - присоединиться к комнате
func (s *LiveKitService) JoinRoom(roomID, userUID string) (*chat.LiveRoom, string, error) {
	var room chat.LiveRoom
	if err := s.db.Preload("Host").Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return nil, "", errors.New("room not found")
	}

	if room.Status != chat.LiveRoomStatusActive {
		return nil, "", errors.New("room is not active")
	}

	// Проверить является ли пользователь участником чата
	var member chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", room.ThreadID, userUID).First(&member).Error; err != nil {
		return nil, "", errors.New("user is not a member of this thread")
	}

	// Проверить заблокирован ли вход
	if room.IsLocked {
		// Проверить в whitelist
		allowed := false
		for _, uid := range room.AllowedUIDs {
			if uid == userUID {
				allowed = true
				break
			}
		}
		if !allowed && userUID != room.HostUID {
			return nil, "", errors.New("room is locked")
		}
	}

	// Проверить лимит участников
	if room.ParticipantCount >= room.MaxParticipants {
		return nil, "", errors.New("room is full")
	}

	// Определить права (хост/со-хост могут управлять)
	canPublish := true
	canAdmin := member.Role == chat.MemberRoleHost || member.Role == chat.MemberRoleCoHost

	// Сгенерировать токен
	token, err := s.generateToken(room.RoomID, userUID, canAdmin)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Увеличить счетчик (примерно, точный подсчет через webhooks)
	s.db.Model(&room).Update("participant_count", gorm.Expr("participant_count + 1"))

	// Reload room with updated participant count for broadcast
	s.db.Preload("Host").First(&room, room.ID)

	_ = canPublish // используем в будущем для mute по умолчанию

	return &room, token, nil
}

// EndRoom - завершить комнату (только хост, или любой участник в DM)
func (s *LiveKitService) EndRoom(roomID, userUID string) error {
	var room chat.LiveRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return errors.New("room not found")
	}

	// Проверить права
	var member chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", room.ThreadID, userUID).First(&member).Error; err != nil {
		return errors.New("user is not a member of this thread")
	}

	// В DM (type=1) любой участник может завершить звонок
	var thread chat.Thread
	if err := s.db.Where("thread_id = ?", room.ThreadID).First(&thread).Error; err != nil {
		return errors.New("thread not found")
	}

	isDM := thread.Type != nil && *thread.Type == 1
	if !isDM && member.Role != chat.MemberRoleHost && member.Role != chat.MemberRoleCoHost {
		return errors.New("only host or co-host can end room")
	}

	// Завершить комнату
	now := utils.CustomTime{Time: time.Now()}
	return s.db.Model(&room).Updates(map[string]interface{}{
		"status":   chat.LiveRoomStatusEnded,
		"ended_at": now,
	}).Error
}

// LockRoom - заблокировать/разблокировать вход
func (s *LiveKitService) LockRoom(roomID, userUID string, lock bool) error {
	var room chat.LiveRoom
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return errors.New("room not found")
	}

	// Проверить права
	var member chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid = ?", room.ThreadID, userUID).First(&member).Error; err != nil {
		return errors.New("user is not a member of this thread")
	}

	// В DM (type=1) любой участник может управлять комнатой
	var thread chat.Thread
	if err := s.db.Where("thread_id = ?", room.ThreadID).First(&thread).Error; err != nil {
		return errors.New("thread not found")
	}

	isDM := thread.Type != nil && *thread.Type == 1
	if !isDM && member.Role != chat.MemberRoleHost && member.Role != chat.MemberRoleCoHost {
		return errors.New("only host or co-host can lock room")
	}

	return s.db.Model(&room).Update("is_locked", lock).Error
}

// GetActiveRoom - получить активную комнату в чате
func (s *LiveKitService) GetActiveRoom(threadID string) (*chat.LiveRoom, error) {
	var rooms []chat.LiveRoom
	if err := s.db.Preload("Host").Where("thread_id = ? AND status = ?", threadID, chat.LiveRoomStatusActive).Limit(1).Find(&rooms).Error; err != nil {
		return nil, err
	}
	if len(rooms) == 0 {
		return nil, nil
	}
	return &rooms[0], nil
}

// GetLiveKitURL - получить URL сервера LiveKit
func (s *LiveKitService) GetLiveKitURL() string {
	return s.config.URL
}

// GetRoomByID - получить комнату по ID
func (s *LiveKitService) GetRoomByID(roomID string) (*chat.LiveRoom, error) {
	var room chat.LiveRoom
	if err := s.db.Preload("Host").Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

// LeaveRoom - выйти из комнаты, вернуть true если комната закрылась
func (s *LiveKitService) LeaveRoom(roomID, userUID string) (*chat.LiveRoom, bool, error) {
	var room chat.LiveRoom
	if err := s.db.Where("room_id = ? AND status = ?", roomID, chat.LiveRoomStatusActive).First(&room).Error; err != nil {
		return nil, false, errors.New("room not found or already ended")
	}

	// Уменьшаем счетчик
	newCount := room.ParticipantCount - 1
	if newCount < 0 {
		newCount = 0
	}

	// Если комната пустая - завершаем её
	if newCount == 0 {
		now := utils.CustomTime{Time: time.Now()}
		s.db.Model(&room).Updates(map[string]interface{}{
			"status":            chat.LiveRoomStatusEnded,
			"ended_at":          now,
			"participant_count": 0,
		})
		s.db.Preload("Host").First(&room, room.ID)
		return &room, true, nil
	}

	// Просто уменьшаем счетчик
	s.db.Model(&room).Update("participant_count", newCount)
	return &room, false, nil
}

// generateToken - генерация JWT токена для LiveKit
func (s *LiveKitService) generateToken(roomID, userUID string, canAdmin bool) (string, error) {
	at := auth.NewAccessToken(s.config.APIKey, s.config.APISecret)

	canPublish := true
	canSubscribe := true

	grant := &auth.VideoGrant{
		RoomJoin:     true,
		Room:         roomID,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}

	if canAdmin {
		grant.RoomAdmin = true
	}

	at.AddGrant(grant).
		SetIdentity(userUID).
		SetValidFor(24 * time.Hour) // Токен на 24 часа

	return at.ToJWT()
}
