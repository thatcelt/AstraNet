package blog

import (
	"encoding/json"

	"github.com/AugustLigh/GoMino/internal/models/utils"
)

type CreateBlogRequest struct {
	Title               string            `json:"title"`
	Content             string            `json:"content"`
	MediaList           []utils.MediaItem `json:"mediaList"`
	BackgroundMediaList []utils.MediaItem `json:"backgroundMediaList"`
	Extensions          *BlogRequestExtensions `json:"extensions,omitempty"`
}

type BlogRequestExtensions struct {
	BackgroundMediaList []utils.MediaItem `json:"backgroundMediaList,omitempty"`
	Bm                  json.RawMessage   `json:"bm,omitempty"` // Amino format: [100, "url", null]
}

// GetBackgroundMediaList возвращает backgroundMediaList из extensions или напрямую
func (r *CreateBlogRequest) GetBackgroundMediaList() []utils.MediaItem {
	// Сначала проверяем напрямую в теле запроса
	if len(r.BackgroundMediaList) > 0 {
		return r.BackgroundMediaList
	}

	// Затем проверяем в extensions
	if r.Extensions != nil {
		// Проверяем extensions.backgroundMediaList
		if len(r.Extensions.BackgroundMediaList) > 0 {
			return r.Extensions.BackgroundMediaList
		}

		// Проверяем extensions.bm (Amino формат)
		if len(r.Extensions.Bm) > 0 {
			var bm utils.MediaItem
			if err := json.Unmarshal(r.Extensions.Bm, &bm); err == nil {
				return []utils.MediaItem{bm}
			}
		}
	}

	return nil
}

type FeatureBlogRequest struct {
	Days int `json:"days"` // 1, 2, 3, or 4
}

type VoteBlogRequest struct {
	Value int `json:"value"` // 1 = like, 0 = unlike
}

type BlogCommentRequest struct {
	Content   string  `json:"content"`
	StickerID *string `json:"stickerId"`
	Type      int     `json:"type"`
	RespondTo *string `json:"respondTo,omitempty"`
}

type ModerationRequest struct {
	Reason string `json:"reason"`
}
