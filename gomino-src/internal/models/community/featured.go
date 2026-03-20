package community

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// Language segments for featured communities
const (
	SegmentRussian  = "ru"
	SegmentEnglish  = "en"
	SegmentSpanish  = "es"
	SegmentArabic   = "ar"
)

// ValidSegments contains all valid language segments
var ValidSegments = []string{
	SegmentRussian,
	SegmentEnglish,
	SegmentSpanish,
	SegmentArabic,
}

// FeaturedCommunity represents a community in the featured collection
type FeaturedCommunity struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	NdcID       int              `json:"ndcId" gorm:"index:idx_featured_segment,unique;not null"`
	Segment     string           `json:"segment" gorm:"index:idx_featured_segment,unique;size:10;not null"`
	Position    int              `json:"position" gorm:"default:0"`
	AddedBy     string           `json:"addedBy" gorm:"not null"`
	CreatedTime utils.CustomTime `json:"createdTime"`

	// Preloaded community data (not stored, populated on read)
	Community *Community `json:"community,omitempty" gorm:"-"`
}

func (FeaturedCommunity) TableName() string {
	return "featured_communities"
}

// BeforeCreate sets timestamps
func (f *FeaturedCommunity) BeforeCreate(tx *gorm.DB) error {
	if f.CreatedTime.IsZero() {
		f.CreatedTime = utils.CustomTime{Time: time.Now()}
	}
	return nil
}

// IsValidSegment checks if the segment is valid
func IsValidSegment(segment string) bool {
	for _, s := range ValidSegments {
		if s == segment {
			return true
		}
	}
	return false
}
