package user

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
)

// GlobalBan represents a global ban record (banned across all communities)
// Only Astranet team can create/remove global bans
type GlobalBan struct {
	ID          uint             `gorm:"primaryKey" json:"id"`
	UID         string           `gorm:"uniqueIndex;not null" json:"uid"`
	BannedByUID string           `gorm:"index" json:"bannedByUid"`
	Reason      string           `gorm:"type:text" json:"reason"`
	BannedAt    utils.CustomTime `json:"bannedAt"`
	IsActive    bool             `gorm:"default:true;index" json:"isActive"`

	// Relations
	BannedUser Author `gorm:"foreignKey:UID;references:UID" json:"bannedUser,omitempty"`
	BannedBy   Author `gorm:"foreignKey:BannedByUID;references:UID" json:"bannedBy,omitempty"`
}

func (GlobalBan) TableName() string {
	return "global_bans"
}

func (g *GlobalBan) BeforeCreate() error {
	if g.BannedAt.IsZero() {
		g.BannedAt = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
