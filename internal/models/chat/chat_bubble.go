package chat

import (
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// ChatBubble представляет скин пузыря сообщений
type ChatBubble struct {
	ID              uint             `json:"id" gorm:"primaryKey"`
	BubbleID        string           `json:"bubbleId" gorm:"uniqueIndex;not null;size:100"`
	Name            string           `json:"name" gorm:"size:200"`
	ResourceURL     string           `json:"resourceUrl" gorm:"column:resource_url;size:500"`
	Icon            string           `json:"icon" gorm:"size:500"`
	BackgroundPath  string           `json:"backgroundPath,omitempty" gorm:"size:500"`
	Status          int              `json:"status" gorm:"default:0"`
	OwnershipStatus *int             `json:"ownershipStatus,omitempty"`
	Version         int              `json:"version" gorm:"default:1"`
	// Nine-slice insets for stretching
	SliceLeft   float64          `json:"sliceLeft" gorm:"default:40"`
	SliceTop    float64          `json:"sliceTop" gorm:"default:40"`
	SliceRight  float64          `json:"sliceRight" gorm:"default:40"`
	SliceBottom float64          `json:"sliceBottom" gorm:"default:40"`
	CreatedAt   utils.CustomTime `json:"createdTime"`
	UpdatedAt   utils.CustomTime `json:"-"`
	DeletedAt   gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (ChatBubble) TableName() string {
	return "chat_bubbles"
}

// UserChatBubble связь пользователя с его пузырями (инвентарь)
type UserChatBubble struct {
	ID        uint             `json:"id" gorm:"primaryKey"`
	UserID    string           `json:"userId" gorm:"index;not null;size:100"`
	BubbleID  string           `json:"bubbleId" gorm:"index;not null;size:100"`
	Status    int              `json:"status" gorm:"default:1"` // 1 = active
	CreatedAt utils.CustomTime `json:"-"`
	DeletedAt gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (UserChatBubble) TableName() string {
	return "user_chat_bubbles"
}

// UserFrameCollection связь пользователя с его рамками аватара (инвентарь)
type UserFrameCollection struct {
	ID        uint             `json:"id" gorm:"primaryKey"`
	UserID    string           `json:"userId" gorm:"index;not null;size:100"`
	FrameID   string           `json:"frameId" gorm:"index;not null;size:100"`
	Status    int              `json:"status" gorm:"default:1"` // 1 = active
	CreatedAt utils.CustomTime `json:"-"`
	DeletedAt gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (UserFrameCollection) TableName() string {
	return "user_frame_collections"
}
