package community

import (
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
)

type ThemePack struct {
	ThemeColor      string `json:"themeColor"`
	ThemeSideImage  string `json:"themeSideImage,omitempty"`
	ThemeUpperImage string `json:"themeUpperImage,omitempty"`
	Cover           string `json:"cover,omitempty"`
}

type Community struct {
	Original_link string `json:"original_link" gorm:"not null"`
	CreatedTime          utils.CustomTime        `json:"createdTime"`
	TemplateId           int                     `json:"templateId" gorm:"default:1"`
	ThemePack            *ThemePack              `json:"themePack" gorm:"serializer:json;type:json"`
	Extensions           *map[string]interface{} `json:"extensions" gorm:"serializer:json;type:json"`
	Name                 string                  `json:"name" gorm:"not null"`
	Endpoint             string                  `json:"endpoint" gorm:"not null;uniqueIndex"`
	UpdatedTime          utils.CustomTime        `json:"updatedTime"`
	Icon                 string                  `json:"icon"`

	Link         string           `json:"link" gorm:"not null;uniqueIndex"`
	ActiveInfo   map[string]any   `json:"activeInfo" gorm:"serializer:json;type:json"`
	NdcId        int              `json:"ndcId" gorm:"not null;uniqueIndex"`
	ModifiedTime utils.CustomTime `json:"modifiedTime"`
	Status       int              `json:"status" gorm:"default:0"`
	JoinType     int              `json:"joinType" gorm:"default:0"`
	// advancedSettings //TODO: тоже по позже
	Tagline           *string               `json:"tagline"`
	Content           *string               `json:"content"`
	MediaList         *[]utils.MediaItem    `json:"mediaList" gorm:"serializer:json;type:json"`
	CommunityHeat     int                   `json:"communityHeat"`
	PrimaryLanguage   string                `json:"primaryLanguage"`
	MembersCount      int                   `json:"membersCount"`
	Keywords          *string               `json:"keywords"`
	ProbationStatus   int                   `json:"probationStatus"`
	ListedStatus      int                   `json:"listedStatus"`
	Agent             user.DetailedAuthor   `json:"agent" gorm:"serializer:json;type:json"`
	Searchable        bool                  `json:"searchable" gorm:"default:true"`
	CommunityHeadList []user.DetailedAuthor `json:"communityHeadList" gorm:"serializer:json;type:json"`
	MembershipStatus  int                  `json:"membershipStatus" gorm:"-"`
}
