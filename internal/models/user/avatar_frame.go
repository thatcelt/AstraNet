package user

import (
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// AvatarFrame представляет рамку аватара пользователя
type AvatarFrame struct {
	ID              uint             `json:"id" gorm:"primaryKey"`
	Status          int              `json:"status"`
	OwnershipStatus *int             `json:"ownershipStatus,omitempty"`
	Version         int              `json:"version"`
	ResourceURL     string           `json:"resourceUrl" gorm:"column:resource_url"`
	Name            string           `json:"name"`
	Icon            string           `json:"icon"`
	FrameType       int              `json:"frameType"`
	FrameID         string           `json:"frameId" gorm:"uniqueIndex;not null"`
	CreatedAt       utils.CustomTime `json:"createdAt"`
	UpdatedAt       utils.CustomTime `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (AvatarFrame) TableName() string {
	return "avatar_frames"
}
