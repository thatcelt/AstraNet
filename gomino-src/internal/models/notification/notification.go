package notification

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// NotificationType defines the type of notification
type NotificationType int

const (
	NotificationNewPost       NotificationType = 1 // New post from followed user
	NotificationNewChat       NotificationType = 2 // New public chat from followed user
	NotificationNewFollower   NotificationType = 3 // New follower
	NotificationLiveRoomStart NotificationType = 4 // Live room started in a chat
)

// ObjectType defines what the notification is about
type ObjectType int

const (
	ObjectTypeUser ObjectType = 0 // User profile
	ObjectTypeBlog ObjectType = 1 // Blog post
	ObjectTypeChat ObjectType = 2 // Chat thread
)

// Notification represents a notification entity
type Notification struct {
	ID           string           `json:"notificationId" gorm:"primaryKey"`
	RecipientUID string           `json:"recipientUid" gorm:"index:idx_recipient_read;not null"`
	ActorUID     string           `json:"actorUid" gorm:"index"`
	Actor        *user.Author     `json:"author,omitempty" gorm:"foreignKey:ActorUID,NdcID;references:UID,NdcID"`
	Type         NotificationType `json:"type"`
	NdcID        int              `json:"ndcId" gorm:"index;default:0"` // 0 = global
	ObjectID     string           `json:"objectId,omitempty"`           // blogId, threadId, userId
	ObjectType   ObjectType       `json:"objectType"`                   // 0=user, 1=blog, 2=chat
	Title        string           `json:"title,omitempty"`
	Content      string           `json:"content,omitempty"`
	Icon         string           `json:"icon,omitempty"`
	IsRead       bool             `json:"isRead" gorm:"index:idx_recipient_read;default:false"`
	CreatedTime  utils.CustomTime `json:"createdTime"`

	// GORM metadata
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Notification) TableName() string {
	return "notifications"
}

// BeforeCreate - GORM hook, called before creating a record
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}
	if n.CreatedTime.IsZero() {
		n.CreatedTime = now
	}
	return nil
}

// NotificationResponse is the API response wrapper
type NotificationResponse struct {
	NotificationList []Notification `json:"notificationList"`
}

// UnreadCountResponse is the response for unread count endpoint
type UnreadCountResponse struct {
	UnreadCount int64 `json:"unreadCount"`
}
