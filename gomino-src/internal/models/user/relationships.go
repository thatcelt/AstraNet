package user

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// UserFollow - модель для хранения подписок пользователей
type UserFollow struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	FollowerUID string    `gorm:"index:idx_follow,unique" json:"followerUid"` // Кто подписался
	TargetUID   string    `gorm:"index:idx_follow,unique" json:"targetUid"`   // На кого подписались
	NdcID       int       `gorm:"index:idx_follow,unique;default:0" json:"ndcId"` // ID сообщества (0 = глобал)
	CreatedAt   time.Time `json:"createdTime"`
}

// TableName - имя таблицы в БД
func (UserFollow) TableName() string {
	return "user_follows"
}

// Comment - модель для комментариев (на стене и не только)
type Comment struct {
	ID             string           `gorm:"primaryKey" json:"commentId"`
	ParentID       string           `gorm:"index" json:"parentId"`     // ID родительского объекта (UserProfile UID, BlogID, etc)
	ParentType     int              `gorm:"index" json:"parentType"`   // Тип родителя (0 - UserProfile, 1 - Blog...)
	NdcID          int              `gorm:"index;default:0" json:"ndcId"` // ID сообщества (0 = глобал)
	Content        string           `gorm:"type:text" json:"content"`
	AuthorUID      string           `gorm:"index" json:"-"`
	Author         DetailedAuthor   `gorm:"foreignKey:AuthorUID,NdcID;references:UID,NdcID" json:"author"`
	CreatedTime    utils.CustomTime `json:"createdTime"`
	ModifiedTime   utils.CustomTime `json:"modifiedTime"`
	LikeCount      int              `gorm:"default:0" json:"votesCount"`
	SubcommentsCount int            `gorm:"default:0" json:"subcommentsCount"`
	
	// Поля для иерархии комментариев (ответы)
	RootCommentID  *string          `gorm:"index" json:"respondTo,omitempty"` // Если это ответ, то ID корневого комментария
	
	CreatedAt      time.Time        `gorm:"autoCreateTime" json:"-"`
	UpdatedAt      time.Time        `gorm:"autoUpdateTime" json:"-"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"-"`
}

// TableName - имя таблицы в БД
func (Comment) TableName() string {
	return "comments"
}

func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}
	if c.CreatedTime.IsZero() {
		c.CreatedTime = now
	}
	if c.ModifiedTime.IsZero() {
		c.ModifiedTime = now
	}
	return nil
}
