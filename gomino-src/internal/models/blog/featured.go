package blog

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// FeaturedPost представляет пост в подборке сообщества
type FeaturedPost struct {
	ID             uint             `gorm:"primaryKey" json:"-"`
	BlogID         string           `gorm:"not null;index" json:"blogId"`
	NdcID          int              `gorm:"not null;index" json:"ndcId"`
	FeaturedByUID  string           `gorm:"not null" json:"featuredByUid"`
	FeaturedUntil  utils.CustomTime `gorm:"not null;index" json:"featuredUntil"`
	CreatedTime    utils.CustomTime `json:"createdTime"`

	// Relations
	Blog *Blog `gorm:"foreignKey:BlogID;references:BlogID" json:"blog,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (FeaturedPost) TableName() string {
	return "featured_posts"
}

func (f *FeaturedPost) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}
	if f.CreatedTime.IsZero() {
		f.CreatedTime = now
	}
	return nil
}
