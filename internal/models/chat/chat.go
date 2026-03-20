package chat

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

type ThreadExtensions struct {
	Announcement                 *string       `json:"announcement,omitempty"`
	CoHost                       []string      `json:"coHost"`
	Language                     *string       `json:"language,omitempty"`
	MembersCanInvite             *bool         `json:"membersCanInvite,omitempty"`
	BM                           []interface{} `json:"bm,omitempty"`
	LastMembersSummaryUpdateTime *int64        `json:"lastMembersSummaryUpdateTime,omitempty"`
	FansOnly                     *bool         `json:"fansOnly,omitempty"`
	PinAnnouncement              *bool         `json:"pinAnnouncement,omitempty"`
}

// Value - реализация интерфейса driver.Valuer для GORM
func (te ThreadExtensions) Value() (driver.Value, error) {
	return json.Marshal(te)
}

// Scan - реализация интерфейса sql.Scanner для GORM
func (te *ThreadExtensions) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to unmarshal ThreadExtensions value: unsupported type")
	}

	return json.Unmarshal(bytes, te)
}

type Thread struct {
	ID uint `gorm:"primaryKey" json:"-"`

	ThreadID  string  `gorm:"uniqueIndex;not null" json:"threadId"`
	Title     *string `gorm:"size:500" json:"title,omitempty"`
	Content   *string `gorm:"type:text" json:"content,omitempty"`
	Icon      *string `gorm:"size:500" json:"icon,omitempty"`
	Keywords  *string `gorm:"size:500" json:"keywords,omitempty"`
	Type      *int    `gorm:"default:0" json:"type,omitempty"`
	Status    *int    `gorm:"default:0" json:"status,omitempty"`
	Condition *int    `gorm:"default:0" json:"condition,omitempty"`

	IsPinned         *bool `gorm:"default:false" json:"isPinned,omitempty"`
	NeedHidden       *bool `gorm:"default:false" json:"needHidden,omitempty"`
	AlertOption      *int  `gorm:"default:0" json:"alertOption,omitempty"`
	MembershipStatus *int  `gorm:"default:0" json:"membershipStatus,omitempty"`

	MembersCount *int `gorm:"default:0" json:"membersCount,omitempty"`
	MembersQuota *int `gorm:"default:100" json:"membersQuota,omitempty"`

	CreatedTime        utils.CustomTime `gorm:"not null" json:"createdTime"`
	ModifiedTime       utils.CustomTime `gorm:"not null" json:"modifiedTime"`
	LatestActivityTime utils.CustomTime `json:"latestActivityTime,omitempty"`
	LastReadTime       utils.CustomTime `json:"lastReadTime,omitempty"`

	UID          string  `gorm:"not null;index" json:"uid,omitempty"`
	NdcID        *int    `gorm:"index" json:"ndcId,omitempty"`
	StrategyInfo *string `gorm:"size:255" json:"strategyInfo,omitempty"`

	UserAddedTopicList []string          `gorm:"type:json;serializer:json" json:"userAddedTopicList,omitempty"`
	MembersSummary     []interface{}     `gorm:"type:json;serializer:json" json:"membersSummary"`
	Extensions         *ThreadExtensions `gorm:"type:json;serializer:json" json:"extensions,omitempty"`

	// Связи (загружаются через Preload)
	Author             user.DetailedAuthor `gorm:"foreignKey:UID;references:UID" json:"author,omitempty"`
	Messages           []Message           `gorm:"foreignKey:ThreadID;references:ThreadID" json:"-"`
	LastMessageSummary *Message            `gorm:"-" json:"lastMessageSummary,omitempty"` // Computed field

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName - имя таблицы в БД
func (Thread) TableName() string {
	return "threads"
}

// BeforeCreate - хук перед созданием записи
func (t *Thread) BeforeCreate(tx *gorm.DB) error {
	now := utils.CustomTime{Time: time.Now()}

	if t.CreatedTime.IsZero() {
		t.CreatedTime = now
	}
	if t.ModifiedTime.IsZero() {
		t.ModifiedTime = now
	}
	if t.LatestActivityTime.IsZero() {
		t.LatestActivityTime = now
	}

	return nil
}

// BeforeUpdate - хук перед обновлением записи
func (t *Thread) BeforeUpdate(tx *gorm.DB) error {
	t.ModifiedTime = utils.CustomTime{Time: time.Now()}
	return nil
}
