package chat

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// MemberRole - роль участника в чате
type MemberRole int

const (
	MemberRoleMember MemberRole = 0  // Обычный участник
	MemberRoleCoHost MemberRole = 10 // Помощник
	MemberRoleHost   MemberRole = 20 // Владелец/организатор
)

// ThreadMember - модель участника чата
type ThreadMember struct {
	ID uint `gorm:"primaryKey" json:"-"`

	ThreadID string     `gorm:"not null;index:idx_thread_user,unique" json:"threadId"`
	UserUID  string     `gorm:"not null;index:idx_thread_user,unique;index" json:"uid"`
	Role     MemberRole `gorm:"default:0" json:"role"`

	AlertOption  int              `gorm:"default:0" json:"alertOption"`
	LastReadTime utils.CustomTime `json:"lastReadTime"`
	JoinedTime   utils.CustomTime `gorm:"not null" json:"joinedTime"`

	// Связи
	User   user.Author `gorm:"foreignKey:UserUID;references:UID" json:"user"`
	Thread *Thread     `gorm:"foreignKey:ThreadID;references:ThreadID" json:"-"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"-"`
}

func (ThreadMember) TableName() string {
	return "thread_members"
}

func (m *ThreadMember) BeforeCreate(tx *gorm.DB) error {
	if m.JoinedTime.IsZero() {
		m.JoinedTime = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
