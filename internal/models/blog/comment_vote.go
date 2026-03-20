package blog

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// CommentVote представляет голос (лайк) за комментарий
type CommentVote struct {
	ID          uint             `gorm:"primaryKey" json:"-"`
	CommentID   string           `gorm:"not null;index:idx_comment_user,unique" json:"commentId"`
	UserUID     string           `gorm:"not null;index:idx_comment_user,unique" json:"uid"`
	VoteType    int              `gorm:"not null" json:"voteType"` // 1 = upvote, 0 = removed
	CreatedTime utils.CustomTime `json:"createdTime"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (CommentVote) TableName() string {
	return "comment_votes"
}

func (v *CommentVote) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}
	if v.CreatedTime.IsZero() {
		v.CreatedTime = now
	}
	return nil
}
