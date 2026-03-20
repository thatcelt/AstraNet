package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/notification"
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"github.com/AugustLigh/GoMino/internal/ws"
	"gorm.io/gorm"
)

const WsEventReadModeChange = 450

// Allowed durations for global read mode (in minutes)
var allowedDurations = map[int]bool{
	30:   true, // 30 min
	60:   true, // 1 hour
	120:  true, // 2 hours
	1440: true, // 24 hours
	4320: true, // 3 days
}

var (
	ErrInvalidDuration   = errors.New("invalid duration")
	ErrReadModeNotFound  = errors.New("read mode not found")
	ErrAlreadyInReadMode = errors.New("user is already in read mode")
)

// ReadModeService handles read mode business logic
type ReadModeService struct {
	db  *gorm.DB
	hub *ws.Hub
}

// NewReadModeService creates a new ReadModeService instance
func NewReadModeService(db *gorm.DB, hub *ws.Hub) *ReadModeService {
	return &ReadModeService{db: db, hub: hub}
}

// EnableCommunityReadMode puts a user into read mode within a community.
// durationMinutes <= 0 means indefinite (admin must manually disable).
func (s *ReadModeService) EnableCommunityReadMode(requesterUID, targetUID string, ndcID int, reason string, durationMinutes int) error {
	permSvc := NewPermissionService(s.db)
	if !permSvc.CanPerform(requesterUID, targetUID, ndcID, "", user.RoleCurator, false) {
		return ErrPermissionDenied
	}

	// Validate duration if provided
	if durationMinutes > 0 && !allowedDurations[durationMinutes] {
		return ErrInvalidDuration
	}

	// Check if already in community read mode
	var existing user.ReadMode
	err := s.db.Where("uid = ? AND scope = 'community' AND ndc_id = ? AND is_active = true", targetUID, ndcID).First(&existing).Error
	if err == nil {
		return ErrAlreadyInReadMode
	}

	rm := &user.ReadMode{
		UID:      targetUID,
		SetByUID: requesterUID,
		Scope:    "community",
		NdcID:    ndcID,
		Reason:   reason,
		IsActive: true,
	}

	if durationMinutes > 0 {
		expiresAt := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
		rm.ExpiresAt = &expiresAt
	}

	if err := s.db.Create(rm).Error; err != nil {
		return fmt.Errorf("failed to enable community read mode: %w", err)
	}

	// Log moderation action
	s.logAction(ndcID, requesterUID, targetUID, community.OpTypeReadModeEnable, reason)

	// Notify user
	s.notifyReadModeChange(targetUID, requesterUID, ndcID, true, reason, rm.ExpiresAt)

	// Broadcast via WebSocket
	s.broadcastReadModeChange(targetUID, rm)

	return nil
}

// DisableCommunityReadMode lifts read mode from a user in a community
func (s *ReadModeService) DisableCommunityReadMode(requesterUID, targetUID string, ndcID int) error {
	permSvc := NewPermissionService(s.db)
	if !permSvc.CanPerform(requesterUID, targetUID, ndcID, "", user.RoleCurator, false) {
		return ErrPermissionDenied
	}

	result := s.db.Model(&user.ReadMode{}).
		Where("uid = ? AND scope = 'community' AND ndc_id = ? AND is_active = true", targetUID, ndcID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to disable community read mode: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrReadModeNotFound
	}

	s.logAction(ndcID, requesterUID, targetUID, community.OpTypeReadModeDisable, "")
	s.notifyReadModeChange(targetUID, requesterUID, ndcID, false, "", nil)
	s.broadcastReadModeChange(targetUID, &user.ReadMode{
		UID:      targetUID,
		Scope:    "community",
		NdcID:    ndcID,
		IsActive: false,
	})

	return nil
}

// EnableGlobalReadMode puts a user into global read mode (Astranet only)
func (s *ReadModeService) EnableGlobalReadMode(requesterUID, targetUID string, durationMinutes int, reason string) error {
	if !isTeamAstranet(s.db, requesterUID) {
		return ErrPermissionDenied
	}

	if !allowedDurations[durationMinutes] {
		return ErrInvalidDuration
	}

	// Check if already in global read mode
	var existing user.ReadMode
	err := s.db.Where("uid = ? AND scope = 'global' AND is_active = true", targetUID).First(&existing).Error
	if err == nil {
		return ErrAlreadyInReadMode
	}

	expiresAt := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
	rm := &user.ReadMode{
		UID:       targetUID,
		SetByUID:  requesterUID,
		Scope:     "global",
		NdcID:     0,
		Reason:    reason,
		IsActive:  true,
		ExpiresAt: &expiresAt,
	}

	if err := s.db.Create(rm).Error; err != nil {
		return fmt.Errorf("failed to enable global read mode: %w", err)
	}

	s.notifyReadModeChange(targetUID, requesterUID, 0, true, reason, &expiresAt)
	s.broadcastReadModeChange(targetUID, rm)

	return nil
}

// DisableGlobalReadMode lifts global read mode from a user (Astranet only)
func (s *ReadModeService) DisableGlobalReadMode(requesterUID, targetUID string) error {
	if !isTeamAstranet(s.db, requesterUID) {
		return ErrPermissionDenied
	}

	result := s.db.Model(&user.ReadMode{}).
		Where("uid = ? AND scope = 'global' AND is_active = true", targetUID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to disable global read mode: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrReadModeNotFound
	}

	s.notifyReadModeChange(targetUID, requesterUID, 0, false, "", nil)
	s.broadcastReadModeChange(targetUID, &user.ReadMode{
		UID:      targetUID,
		Scope:    "global",
		NdcID:    0,
		IsActive: false,
	})

	return nil
}

// IsInReadMode checks whether a user is in read mode for a given context.
// It checks both global and community-level read modes.
// Expired global read modes are auto-deactivated.
func (s *ReadModeService) IsInReadMode(uid string, ndcID int) (bool, *user.ReadMode) {
	now := time.Now()

	// Check global read mode first
	var globalRM user.ReadMode
	err := s.db.Where("uid = ? AND scope = 'global' AND is_active = true", uid).First(&globalRM).Error
	if err == nil {
		// Auto-expire if ExpiresAt has passed
		if globalRM.ExpiresAt != nil && globalRM.ExpiresAt.Before(now) {
			s.db.Model(&globalRM).Update("is_active", false)
			// Don't return - check community too
		} else {
			return true, &globalRM
		}
	}

	// Check community read mode
	if ndcID > 0 {
		var comRM user.ReadMode
		err := s.db.Where("uid = ? AND scope = 'community' AND ndc_id = ? AND is_active = true", uid, ndcID).First(&comRM).Error
		if err == nil {
			return true, &comRM
		}
	}

	return false, nil
}

// GetReadModeStatus returns all active read modes for a user
func (s *ReadModeService) GetReadModeStatus(uid string) ([]user.ReadMode, error) {
	now := time.Now()

	// Auto-expire any expired global read modes
	s.db.Model(&user.ReadMode{}).
		Where("uid = ? AND scope = 'global' AND is_active = true AND expires_at < ?", uid, now).
		Update("is_active", false)

	var modes []user.ReadMode
	err := s.db.Preload("SetBy").
		Where("uid = ? AND is_active = true", uid).
		Find(&modes).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get read mode status: %w", err)
	}

	return modes, nil
}

// CleanupExpiredReadModes deactivates all expired global read modes and notifies users.
// Called periodically by the background worker.
func (s *ReadModeService) CleanupExpiredReadModes() {
	now := time.Now()

	var expired []user.ReadMode
	err := s.db.Where("is_active = true AND expires_at IS NOT NULL AND expires_at < ?", now).Find(&expired).Error
	if err != nil {
		log.Printf("Failed to find expired read modes: %v", err)
		return
	}

	if len(expired) == 0 {
		return
	}

	// Batch deactivate
	s.db.Model(&user.ReadMode{}).
		Where("is_active = true AND expires_at IS NOT NULL AND expires_at < ?", now).
		Update("is_active", false)

	// Notify each affected user
	for _, rm := range expired {
		s.notifyReadModeChange(rm.UID, "", 0, false, "", nil)
		s.broadcastReadModeChange(rm.UID, &user.ReadMode{
			UID:      rm.UID,
			Scope:    "global",
			NdcID:    0,
			IsActive: false,
		})
	}

	log.Printf("Cleaned up %d expired read modes", len(expired))
}

// broadcastReadModeChange sends a WebSocket event to the user about read mode state change
func (s *ReadModeService) broadcastReadModeChange(userUID string, rm *user.ReadMode) {
	if s.hub == nil {
		return
	}

	rmData := map[string]interface{}{
		"isActive": rm.IsActive,
		"scope":    rm.Scope,
		"ndcId":    rm.NdcID,
		"reason":   rm.Reason,
	}
	if rm.ExpiresAt != nil {
		rmData["expiresAt"] = rm.ExpiresAt.Format(time.RFC3339)
	}

	event := map[string]interface{}{
		"t": WsEventReadModeChange,
		"o": map[string]interface{}{
			"readMode": rmData,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.hub.BroadcastToUser(userUID, data)
}

// notifyReadModeChange creates an important notification about read mode change
func (s *ReadModeService) notifyReadModeChange(targetUID, actorUID string, ndcID int, enabled bool, reason string, expiresAt *time.Time) {
	notifType := notification.NotificationReadModeOff
	title := "Read Mode Disabled"
	content := "You can now interact freely."
	if enabled {
		notifType = notification.NotificationReadModeOn
		title = "Read Mode Enabled"
		content = "You are now in read mode. You cannot send messages, post, or interact."
		if reason != "" {
			content += " Reason: " + reason
		}
	}

	n := &notification.Notification{
		ID:             generateUID(),
		RecipientUID:   targetUID,
		ActorUID:       actorUID,
		Type:           notifType,
		NdcID:          ndcID,
		ObjectID:       targetUID,
		ObjectType:     notification.ObjectTypeUser,
		Title:          title,
		Content:        content,
		IsImportant:    true,
		IsAcknowledged: false,
	}

	notifSvc := NewNotificationService(s.db, s.hub)
	if err := notifSvc.CreateNotification(n); err != nil {
		log.Printf("Failed to create read mode notification for user %s: %v", targetUID, err)
	}
}

// logAction logs a moderation action in admin_logs
func (s *ReadModeService) logAction(ndcID int, operatorUID, targetUID string, opType int, note string) {
	logEntry := community.AdminLog{
		NdcID:         ndcID,
		OperatorUID:   operatorUID,
		TargetUID:     targetUID,
		ObjectID:      &targetUID,
		ObjectType:    community.ObjectTypeUser,
		OperationType: opType,
		Note:          note,
		CreatedTime:   utils.CustomTime{Time: time.Now()},
	}
	if err := s.db.Create(&logEntry).Error; err != nil {
		log.Printf("Failed to log read mode action: %v", err)
	}
}
