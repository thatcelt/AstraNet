package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/AugustLigh/GoMino/internal/models/blog"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/notification"
	"github.com/AugustLigh/GoMino/internal/models/user"
	pushService "github.com/AugustLigh/GoMino/internal/service/push"
	"github.com/AugustLigh/GoMino/internal/ws"
	"gorm.io/gorm"
)

// WebSocket event type for notifications
const WsEventNotification = 400

var (
	ErrNotificationNotFound = errors.New("notification not found")
)

// NotificationService handles notification business logic
type NotificationService struct {
	db  *gorm.DB
	hub *ws.Hub
}

// NewNotificationService creates a new NotificationService instance
func NewNotificationService(db *gorm.DB, hub *ws.Hub) *NotificationService {
	return &NotificationService{db: db, hub: hub}
}

// CreateNotification creates a new notification
func (s *NotificationService) CreateNotification(n *notification.Notification) error {
	if n.ID == "" {
		n.ID = generateUID()
	}

	if err := s.db.Create(n).Error; err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Send real-time notification via WebSocket
	s.broadcastNotification(n)

	// Send push notification if enabled
	s.sendPushNotification(n)

	return nil
}

// GetNotifications retrieves notifications for a user
func (s *NotificationService) GetNotifications(userID string, ndcID int, start, size int) ([]notification.Notification, error) {
	var notifications []notification.Notification

	err := s.db.Preload("Actor").
		Where("recipient_uid = ? AND ndc_id = ?", userID, ndcID).
		Order("created_time DESC").
		Offset(start).
		Limit(size).
		Find(&notifications).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}

	return notifications, nil
}

// GetUnreadCount returns the count of unread notifications
func (s *NotificationService) GetUnreadCount(userID string, ndcID int) (int64, error) {
	var count int64
	err := s.db.Model(&notification.Notification{}).
		Where("recipient_uid = ? AND ndc_id = ? AND is_read = ?", userID, ndcID, false).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// MarkAsRead marks a single notification as read
func (s *NotificationService) MarkAsRead(notificationID, userID string) error {
	result := s.db.Model(&notification.Notification{}).
		Where("id = ? AND recipient_uid = ?", notificationID, userID).
		Update("is_read", true)

	if result.Error != nil {
		return fmt.Errorf("failed to mark as read: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (s *NotificationService) MarkAllAsRead(userID string, ndcID int) error {
	result := s.db.Model(&notification.Notification{}).
		Where("recipient_uid = ? AND ndc_id = ? AND is_read = ?", userID, ndcID, false).
		Update("is_read", true)

	if result.Error != nil {
		return fmt.Errorf("failed to mark all as read: %w", result.Error)
	}

	return nil
}

// DeleteNotification deletes a single notification
func (s *NotificationService) DeleteNotification(notificationID, userID string) error {
	result := s.db.Where("id = ? AND recipient_uid = ?", notificationID, userID).
		Delete(&notification.Notification{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete notification: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

// DeleteAllNotifications deletes all notifications for a user
func (s *NotificationService) DeleteAllNotifications(userID string, ndcID int) error {
	result := s.db.Where("recipient_uid = ? AND ndc_id = ?", userID, ndcID).
		Delete(&notification.Notification{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete all notifications: %w", result.Error)
	}

	return nil
}

// broadcastNotification sends real-time notification via WebSocket
func (s *NotificationService) broadcastNotification(n *notification.Notification) {
	if s.hub == nil {
		return
	}

	// Load actor data if not already loaded
	if n.Actor == nil && n.ActorUID != "" {
		var author user.Author
		if err := s.db.First(&author, "uid = ?", n.ActorUID).Error; err == nil {
			n.Actor = &author
		}
	}

	event := map[string]interface{}{
		"t": WsEventNotification,
		"o": map[string]interface{}{
			"notification": n,
			"ndcId":        n.NdcID,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.hub.BroadcastToUser(n.RecipientUID, data)
}

// ========== Notification Generators ==========

// NotifyUserAboutNewFollower creates a notification when someone follows a user
func (s *NotificationService) NotifyUserAboutNewFollower(targetUID, followerUID string) error {
	// Get follower info for notification content
	var follower user.User
	if err := s.db.First(&follower, "uid = ? AND ndc_id = 0", followerUID).Error; err != nil {
		return fmt.Errorf("follower not found: %w", err)
	}

	n := &notification.Notification{
		ID:           generateUID(),
		RecipientUID: targetUID,
		ActorUID:     followerUID,
		Type:         notification.NotificationNewFollower,
		NdcID:        0, // Global
		ObjectID:     followerUID,
		ObjectType:   notification.ObjectTypeUser,
		Title:        "New Follower",
		Content:      fmt.Sprintf("%s started following you", follower.Nickname),
		Icon:         follower.Icon,
	}

	return s.CreateNotification(n)
}

// NotifyFollowersAboutNewPost notifies all followers about a new blog post
func (s *NotificationService) NotifyFollowersAboutNewPost(authorUID string, ndcID int, b *blog.Blog) error {
	// Get author info
	var author user.User
	if err := s.db.First(&author, "uid = ?", authorUID).Error; err != nil {
		return fmt.Errorf("author not found: %w", err)
	}

	// Get all followers of the author
	var follows []user.UserFollow
	if err := s.db.Where("target_uid = ?", authorUID).Find(&follows).Error; err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	// Create notification for each follower
	for _, follow := range follows {
		n := &notification.Notification{
			ID:           generateUID(),
			RecipientUID: follow.FollowerUID,
			ActorUID:     authorUID,
			Type:         notification.NotificationNewPost,
			NdcID:        ndcID,
			ObjectID:     b.BlogID,
			ObjectType:   notification.ObjectTypeBlog,
			Title:        b.Title,
			Content:      fmt.Sprintf("%s published a new post", author.Nickname),
			Icon:         author.Icon,
		}

		// Create notification (ignore errors for individual notifications)
		_ = s.CreateNotification(n)
	}

	return nil
}

// NotifyFollowersAboutNewChat notifies all followers about a new public chat
func (s *NotificationService) NotifyFollowersAboutNewChat(authorUID string, ndcID int, thread *chat.Thread) error {
	// Only notify for public chats (type 0 or 2)
	if thread.Type != nil && *thread.Type != 0 && *thread.Type != 2 {
		return nil
	}

	// Get author info
	var author user.User
	if err := s.db.First(&author, "uid = ?", authorUID).Error; err != nil {
		return fmt.Errorf("author not found: %w", err)
	}

	// Get all followers of the author
	var follows []user.UserFollow
	if err := s.db.Where("target_uid = ?", authorUID).Find(&follows).Error; err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	title := "New Chat"
	if thread.Title != nil {
		title = *thread.Title
	}

	// Create notification for each follower
	for _, follow := range follows {
		n := &notification.Notification{
			ID:           generateUID(),
			RecipientUID: follow.FollowerUID,
			ActorUID:     authorUID,
			Type:         notification.NotificationNewChat,
			NdcID:        ndcID,
			ObjectID:     thread.ThreadID,
			ObjectType:   notification.ObjectTypeChat,
			Title:        title,
			Content:      fmt.Sprintf("%s created a new chat", author.Nickname),
			Icon:         author.Icon,
		}

		_ = s.CreateNotification(n)
	}

	return nil
}

// NotifyThreadMembersAboutLiveRoom notifies all chat members about a live room starting
func (s *NotificationService) NotifyThreadMembersAboutLiveRoom(threadID string, room *chat.LiveRoom, starterUID string) error {
	// Get starter info
	var starter user.User
	if err := s.db.First(&starter, "uid = ?", starterUID).Error; err != nil {
		return fmt.Errorf("starter not found: %w", err)
	}

	// Get thread info for ndcID
	var thread chat.Thread
	if err := s.db.First(&thread, "thread_id = ?", threadID).Error; err != nil {
		return fmt.Errorf("thread not found: %w", err)
	}

	ndcID := 0
	if thread.NdcID != nil {
		ndcID = *thread.NdcID
	}

	// Get all thread members except the starter
	var members []chat.ThreadMember
	if err := s.db.Where("thread_id = ? AND user_uid != ?", threadID, starterUID).Find(&members).Error; err != nil {
		return fmt.Errorf("failed to get members: %w", err)
	}

	title := "Voice Chat"
	if room.Type == chat.LiveRoomTypeVideo {
		title = "Video Chat"
	}
	if room.Title != "" {
		title = room.Title
	}

	threadTitle := "a chat"
	if thread.Title != nil {
		threadTitle = *thread.Title
	}

	// Create notification for each member
	for _, member := range members {
		n := &notification.Notification{
			ID:           generateUID(),
			RecipientUID: member.UserUID,
			ActorUID:     starterUID,
			Type:         notification.NotificationLiveRoomStart,
			NdcID:        ndcID,
			ObjectID:     threadID,
			ObjectType:   notification.ObjectTypeChat,
			Title:        title,
			Content:      fmt.Sprintf("%s started a live room in %s", starter.Nickname, threadTitle),
			Icon:         starter.Icon,
		}

		_ = s.CreateNotification(n)
	}

	return nil
}

// sendPushNotification sends a push notification to the recipient's devices
func (s *NotificationService) sendPushNotification(n *notification.Notification) {
	// Check if push notifications are enabled
	if !IsPushEnabled() {
		return
	}

	pushSvc := GetPushService()
	if pushSvc == nil {
		return
	}

	// Prepare push data
	pushData := pushService.NotificationData{
		Title:          n.Title,
		Body:           n.Content,
		NotificationID: n.ID,
		Type:           int(n.Type),
		ObjectType:     int(n.ObjectType),
		ObjectID:       n.ObjectID,
		NdcID:          n.NdcID,
		ImageURL:       n.Icon,
	}

	// Send push notification asynchronously
	go func() {
		if err := pushSvc.SendPushToUser(n.RecipientUID, pushData); err != nil {
			log.Printf("Failed to send push notification to user %s: %v", n.RecipientUID, err)
		}
	}()
}
