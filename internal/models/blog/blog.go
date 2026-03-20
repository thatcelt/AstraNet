package blog

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

type Blog struct {
	ID        uint   `gorm:"primaryKey" json:"-"`
	BlogID    string `gorm:"uniqueIndex;not null" json:"blogId"`
	Title     string `json:"title"`
	Content   string `gorm:"type:text" json:"content"`

	MediaList *[]utils.MediaItem `json:"mediaList" gorm:"serializer:json;type:json"`

	UID   string `gorm:"not null;index" json:"uid"`
	NdcID int    `gorm:"not null;default:0;index" json:"ndcId"` // 0 for global

	Status     int `gorm:"default:0" json:"status"`
	VotesCount    int `gorm:"default:0" json:"votesCount"`
	CommentsCount int `gorm:"default:0" json:"commentsCount"`

	CreatedTime  utils.CustomTime `json:"createdTime"`
	ModifiedTime utils.CustomTime `json:"modifiedTime"`

	Author user.Author `gorm:"foreignKey:UID,NdcID;references:UID,NdcID" json:"author"`

	// VotedValue - голос текущего пользователя (не хранится в БД, заполняется при запросе)
	VotedValue *int `gorm:"-" json:"votedValue,omitempty"`

	Extensions *BlogExtensions `gorm:"type:json;serializer:json" json:"extensions,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type BlogExtensions struct {
	BackgroundMediaList *[]utils.MediaItem `json:"backgroundMediaList,omitempty"`
}

func (Blog) TableName() string {
	return "blogs"
}

func (b *Blog) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}
	if b.CreatedTime.IsZero() {
		b.CreatedTime = now
	}
	if b.ModifiedTime.IsZero() {
		b.ModifiedTime = now
	}
	return nil
}

func (b *Blog) BeforeUpdate(tx *gorm.DB) error {
	b.ModifiedTime = utils.CustomTime{Time: time.Now()}
	return nil
}
