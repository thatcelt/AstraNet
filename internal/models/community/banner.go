package community

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// Banner represents a banner image displayed at the top of the app
type Banner struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	Segment     string           `json:"segment" gorm:"index:idx_banner_segment;size:10;not null"`
	ImageURL    string           `json:"imageUrl" gorm:"not null"`
	LinkURL     string           `json:"linkUrl" gorm:"not null"`
	Position    int              `json:"position" gorm:"default:0"`
	AddedBy     string           `json:"addedBy" gorm:"not null"`
	CreatedTime utils.CustomTime `json:"createdTime"`
}

func (Banner) TableName() string {
	return "banners"
}

func (b *Banner) BeforeCreate(tx *gorm.DB) error {
	if b.CreatedTime.IsZero() {
		b.CreatedTime = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
