package user

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
)

// UserBlock represents a user-to-user block.
// When user A blocks user B, B cannot message A or join chats where A is the organizer.
// Blocks are global (not scoped to a community).
type UserBlock struct {
	ID         uint             `gorm:"primaryKey" json:"id"`
	UID        string           `gorm:"index:idx_block_pair,unique;not null" json:"uid"`         // blocker
	BlockedUID string           `gorm:"index:idx_block_pair,unique;not null" json:"blockedUid"`   // blocked user
	CreatedAt  utils.CustomTime `json:"createdAt"`

	// Relations
	Blocker     Author `gorm:"foreignKey:UID;references:UID" json:"blocker,omitempty"`
	BlockedUser Author `gorm:"foreignKey:BlockedUID;references:UID" json:"blockedUser,omitempty"`
}

func (UserBlock) TableName() string {
	return "user_blocks"
}

func (b *UserBlock) BeforeCreate() error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
