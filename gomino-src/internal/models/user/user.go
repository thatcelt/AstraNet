package user

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

const (
	RoleBanned   = -1
	RoleMember   = 0
	RoleCurator  = 50
	RoleLeader   = 100
	RoleAgent    = 110
	RoleAstranet = 1000
)

type User struct {
	ID                             uint             `json:"id" gorm:"primaryKey"`
	UID                            string           `json:"uid" gorm:"index:idx_uid_ndcid,unique;not null"`
	Status                         int              `json:"status" gorm:"default:0"`
	MoodSticker                    *string          `json:"moodSticker,omitempty"`
	ItemsCount                     int              `json:"itemsCount" gorm:"default:0"`
	ConsecutiveCheckInDays         *int             `json:"consecutiveCheckInDays,omitempty"`
	ModifiedTime                   utils.CustomTime `json:"modifiedTime"`
	FollowingStatus                int              `json:"followingStatus" gorm:"default:0"`
	OnlineStatus                   int              `json:"onlineStatus" gorm:"default:0"`
	AccountMembershipStatus        int              `json:"accountMembershipStatus" gorm:"default:0"`
	IsGlobal                       bool             `json:"isGlobal" gorm:"default:false"`
	AvatarFrameID                  *string          `json:"avatarFrameId,omitempty"`
	Reputation                     int              `json:"reputation" gorm:"default:0"`
	PostsCount                     int              `json:"postsCount" gorm:"default:0"`
	MembersCount                   int              `json:"membersCount" gorm:"default:0"`
	Nickname                       string           `json:"nickname" gorm:"not null"`
	MediaList                      utils.MediaList  `json:"mediaList,omitempty" gorm:"type:json"`
	Icon                           string           `json:"icon"`
	IsNicknameVerified             bool             `json:"isNicknameVerified" gorm:"default:false"`
	Mood                           *string          `json:"mood,omitempty"`
	Level                          int              `json:"level" gorm:"default:1"`
	NotificationSubscriptionStatus int              `json:"notificationSubscriptionStatus" gorm:"default:0"`
	PushEnabled                    bool             `json:"pushEnabled" gorm:"default:true"`
	MembershipStatus               int              `json:"membershipStatus" gorm:"default:0"`
	Content                        *string          `json:"content,omitempty"`
	JoinedCount                    int              `json:"joinedCount" gorm:"default:0"`
	Role                           int              `json:"role" gorm:"default:0"`
	CommentsCount                  int              `json:"commentsCount" gorm:"default:0"`
	AminoID                        string           `json:"aminoId" gorm:"column:amino_id;uniqueIndex"`
	NdcID                          int              `json:"ndcId" gorm:"index:idx_uid_ndcid,unique;default:0"`
	CreatedTime                    utils.CustomTime `json:"createdTime"`
	StoriesCount                   int              `json:"storiesCount" gorm:"default:0"`
	BlogsCount                     int              `json:"blogsCount" gorm:"default:0"`

	// Связи (загружаются через Preload при необходимости)
	AvatarFrame  *AvatarFrame  `json:"avatarFrame,omitempty" gorm:"foreignKey:AvatarFrameID;references:FrameID"`
	CustomTitles []CustomTitle `json:"customTitles,omitempty" gorm:"foreignKey:UserID;references:UID"`

	// Extensions - JSON поле для дополнительных данных
	Extensions Extensions `json:"extensions" gorm:"type:json"`

	// GORM служебные поля (скрываются от JSON)
	CreatedAt utils.CustomTime `json:"-"`
	UpdatedAt utils.CustomTime `json:"-"`
	DeletedAt gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (User) TableName() string {
	return "users"
}

// BeforeCreate - GORM hook, вызывается перед созданием записи
func (u *User) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}

	if u.CreatedTime.IsZero() {
		u.CreatedTime = now
	}
	if u.ModifiedTime.IsZero() {
		u.ModifiedTime = now
	}

	return nil
}

// BeforeUpdate - GORM hook, вызывается перед обновлением записи
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	u.ModifiedTime = utils.CustomTime{Time: time.Now()}
	return nil
}
