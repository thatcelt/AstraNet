package chat

import (
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// Sticker представляет один стикер
type Sticker struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	StickerID    string           `json:"stickerId" gorm:"uniqueIndex;not null;size:100"`
	CollectionID string           `json:"collectionId" gorm:"index;not null;size:100"`
	Name         string           `json:"name" gorm:"size:200"`
	Icon         string           `json:"icon" gorm:"size:500"`
	MediaValue   string           `json:"mediaValue" gorm:"size:500"`
	Status       int              `json:"status" gorm:"default:0"`
	CreatedAt    utils.CustomTime `json:"createdTime"`
	UpdatedAt    utils.CustomTime `json:"-"`
	DeletedAt    gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (Sticker) TableName() string {
	return "stickers"
}

// StickerCollection представляет стикерпак
type StickerCollection struct {
	ID            uint             `json:"id" gorm:"primaryKey"`
	CollectionID  string           `json:"collectionId" gorm:"uniqueIndex;not null;size:100"`
	Name          string           `json:"name" gorm:"size:200"`
	Icon          string           `json:"icon" gorm:"size:500"`
	Status        int              `json:"status" gorm:"default:0"`
	StickersCount int              `json:"stickersCount" gorm:"default:0"`
	StickerList   []Sticker        `json:"stickerList,omitempty" gorm:"foreignKey:CollectionID;references:CollectionID"`
	CreatedAt     utils.CustomTime `json:"createdTime"`
	UpdatedAt     utils.CustomTime `json:"-"`
	DeletedAt     gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (StickerCollection) TableName() string {
	return "sticker_collections"
}

// UserStickerCollection связь пользователя с его стикерпаками (инвентарь)
type UserStickerCollection struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	UserID       string           `json:"userId" gorm:"index;not null;size:100"`
	CollectionID string           `json:"collectionId" gorm:"index;not null;size:100"`
	Status       int              `json:"status" gorm:"default:1"` // 1 = active
	CreatedAt    utils.CustomTime `json:"-"`
	DeletedAt    gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (UserStickerCollection) TableName() string {
	return "user_sticker_collections"
}
