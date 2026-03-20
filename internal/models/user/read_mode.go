package user

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
)

// ReadMode represents a read-only restriction on a user.
// Scope "global" = app-wide (set by Astranet, has ExpiresAt).
// Scope "community" = within a community (set by Curator+, no time limit).
type ReadMode struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	UID       string           `gorm:"index:idx_rm_uid_active;not null" json:"uid"`
	SetByUID  string           `gorm:"index" json:"setByUid"`
	Scope     string           `gorm:"not null" json:"scope"` // "global" or "community"
	NdcID     int              `gorm:"index;default:0" json:"ndcId"`
	Reason    string           `gorm:"type:text" json:"reason"`
	IsActive  bool             `gorm:"default:true;index:idx_rm_uid_active" json:"isActive"`
	CreatedAt utils.CustomTime `json:"createdAt"`
	ExpiresAt *time.Time       `json:"expiresAt,omitempty"` // nil = indefinite (community scope)

	// Relations
	TargetUser Author `gorm:"foreignKey:UID;references:UID" json:"targetUser,omitempty"`
	SetBy      Author `gorm:"foreignKey:SetByUID;references:UID" json:"setBy,omitempty"`
}

func (ReadMode) TableName() string {
	return "read_modes"
}

func (r *ReadMode) BeforeCreate() error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
