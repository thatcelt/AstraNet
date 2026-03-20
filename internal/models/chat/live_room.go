package chat

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// LiveRoomType - тип комнаты
type LiveRoomType int

const (
	LiveRoomTypeVoice LiveRoomType = 1 // Голосовой чат
	LiveRoomTypeVideo LiveRoomType = 2 // Видео чат
	LiveRoomTypeCinema LiveRoomType = 3 // Кинозал (на будущее)
)

// LiveRoomStatus - статус комнаты
type LiveRoomStatus int

const (
	LiveRoomStatusActive   LiveRoomStatus = 1 // Активна
	LiveRoomStatusEnded    LiveRoomStatus = 2 // Завершена
)

// LiveRoom - модель live комнаты
type LiveRoom struct {
	ID uint `gorm:"primaryKey" json:"-"`

	// Основные поля
	RoomID   string         `gorm:"uniqueIndex;not null" json:"roomId"`
	ThreadID string         `gorm:"not null;index" json:"threadId"`
	Title    string         `gorm:"size:255" json:"title"`
	Type     LiveRoomType   `gorm:"not null" json:"type"`
	Status   LiveRoomStatus `gorm:"default:1" json:"status"`

	// Создатель
	HostUID string `gorm:"not null" json:"hostUid"`

	// Настройки доступа
	IsLocked     bool `gorm:"default:false" json:"isLocked"`     // Запрет на вход
	AllowedUIDs  JSON `gorm:"type:json" json:"allowedUids"`      // Whitelist если isLocked

	// Счетчики
	ParticipantCount int `gorm:"default:0" json:"participantCount"`
	MaxParticipants  int `gorm:"default:100" json:"maxParticipants"`

	// Временные метки
	StartedAt utils.CustomTime  `gorm:"not null" json:"startedAt"`
	EndedAt   *utils.CustomTime `json:"endedAt,omitempty"`

	// Связи
	Host   user.Author `gorm:"foreignKey:HostUID;references:UID" json:"host"`
	Thread *Thread     `gorm:"foreignKey:ThreadID;references:ThreadID" json:"-"`

	// GORM служебные поля
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// JSON тип для хранения массивов
type JSON []string

func (LiveRoom) TableName() string {
	return "live_rooms"
}

// BeforeCreate - хук перед созданием записи
func (r *LiveRoom) BeforeCreate(tx *gorm.DB) error {
	if r.StartedAt.IsZero() {
		r.StartedAt = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
