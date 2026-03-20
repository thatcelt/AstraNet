package blog

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// Vote представляет голос (лайк/дизлайк) за пост
type Vote struct {
	ID          uint             `gorm:"primaryKey" json:"-"`
	BlogID      string           `gorm:"not null;index:idx_blog_user,unique" json:"blogId"`
	UserUID     string           `gorm:"not null;index:idx_blog_user,unique" json:"uid"`
	VoteType    int              `gorm:"not null" json:"voteType"` // 1 = upvote, -1 = downvote, 0 = removed
	CreatedTime utils.CustomTime `json:"createdTime"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Vote) TableName() string {
	return "blog_votes"
}

func (v *Vote) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}
	if v.CreatedTime.IsZero() {
		v.CreatedTime = now
	}
	return nil
}
