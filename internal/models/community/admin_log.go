package community

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// Object Types (Amino standard-ish)
const (
	ObjectTypeUser      = 0
	ObjectTypeBlog      = 1
	ObjectTypeWiki      = 2
	ObjectTypeThread    = 3   // Chat thread
	ObjectTypeQuiz      = 1   // Amino uses 1 for general posts sometimes? User said 1.
	ObjectTypeFile      = 109
	ObjectTypeCommunity = 100 // Example value
)

// Operation Types
const (
	OpTypeBan         = 100
	OpTypeUnban       = 101
	OpTypeWarn        = 102
	OpTypeEdit        = 103 // Example value
	OpTypeFeature     = 104 // Add to featured
	OpTypeUnfeature   = 105 // Remove from featured
	OpTypeHide        = 110 // Hide content (blog/chat)
	OpTypeUnhide      = 111 // Unhide content
	OpTypeGlobalBan       = 120 // Global ban (Astranet only)
	OpTypeGlobalUnban     = 121 // Global unban
	OpTypeReadModeEnable  = 130 // Enable read mode
	OpTypeReadModeDisable = 131 // Disable read mode
)

type AdminLog struct {
	ID            uint             `gorm:"primaryKey" json:"logId"`
	NdcID         int              `gorm:"index" json:"ndcId"`
	OperatorUID   string           `gorm:"index" json:"operatorUid"`
	TargetUID     string           `gorm:"index" json:"targetUid"` // User affected
	ObjectID      *string          `gorm:"index" json:"objectId"`  // Specific content ID (if any)
	ObjectType    int              `json:"objectType"`             // 0=User, 1=Blog...
	OperationType int              `json:"operationType"`          // 100=Ban, etc.
	Note          string           `gorm:"type:text" json:"content"`
	CreatedTime   utils.CustomTime `json:"createdTime"`

	// Relations
	Operator user.Author  `gorm:"foreignKey:OperatorUID,NdcID;references:UID,NdcID" json:"operator"`
	Target   *user.Author `gorm:"foreignKey:TargetUID,NdcID;references:UID,NdcID" json:"targetUser"` // Optional, usually populated if ObjectType=0
}

func (AdminLog) TableName() string {
	return "admin_logs"
}

func (l *AdminLog) BeforeCreate(tx *gorm.DB) error {
	if l.CreatedTime.IsZero() {
		l.CreatedTime = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
