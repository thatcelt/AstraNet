package blog

import (
	"encoding/json"
	"errors"

	"github.com/AugustLigh/GoMino/internal/models/utils"
)

var ErrValidation = errors.New("validation failed")

const (
	MaxBlogTitleLength   = 200
	MaxBlogContentLength = 20000
	MaxMediaItems        = 20
	MaxCommentLength     = 2000
	MaxReasonLength      = 500
)

type CreateBlogRequest struct {
	Title               string            `json:"title"`
	Content             string            `json:"content"`
	MediaList           []utils.MediaItem `json:"mediaList"`
	BackgroundMediaList []utils.MediaItem `json:"backgroundMediaList"`
	Extensions          *BlogRequestExtensions `json:"extensions,omitempty"`
}

func (r *CreateBlogRequest) Validate() error {
	if len(r.Title) == 0 || len(r.Title) > MaxBlogTitleLength {
		return ErrValidation
	}
	if len(r.Content) > MaxBlogContentLength {
		return ErrValidation
	}
	if len(r.MediaList) > MaxMediaItems {
		return ErrValidation
	}
	if len(r.BackgroundMediaList) > MaxMediaItems {
		return ErrValidation
	}
	return nil
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
	Content      string            `json:"content"`
	StickerID    *string           `json:"stickerId"`
	StickerIcon  *string           `json:"stickerIcon"`
	StickerMedia *string           `json:"stickerMediaValue"`
	Type         int               `json:"type"` // 0 = text, 1 = sticker, 2 = media
	MediaList    []utils.MediaItem `json:"mediaList"`
	RespondTo    *string           `json:"respondTo,omitempty"`
}

func (r *BlogCommentRequest) Validate() error {
	switch r.Type {
	case 1: // sticker
		if r.StickerID == nil || *r.StickerID == "" {
			return ErrValidation
		}
	case 2: // media
		if len(r.MediaList) == 0 || len(r.MediaList) > 3 {
			return ErrValidation
		}
	default: // text (0)
		if len(r.Content) == 0 || len(r.Content) > MaxCommentLength {
			return ErrValidation
		}
	}
	if len(r.Content) > MaxCommentLength {
		return ErrValidation
	}
	if len(r.MediaList) > 3 {
		return ErrValidation
	}
	return nil
}

type VoteCommentRequest struct {
	Value int `json:"value"` // 1 = like, 0 = unlike
}

type ModerationRequest struct {
	Reason string `json:"reason"`
}

func (r *ModerationRequest) Validate() error {
	if len(r.Reason) > MaxReasonLength {
		return ErrValidation
	}
	return nil
}
