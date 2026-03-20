package community

import "errors"

var ErrValidation = errors.New("validation failed")

const (
	MaxCommunityNameLength    = 64
	MaxCommunityTaglineLength = 200
	MaxCommunityContentLength = 5000
	MaxCommunityEndpointLength = 32
	MaxKeywordsCount          = 20
	MaxKeywordLength          = 50
	MaxBanReasonLength        = 500
	MaxBannersCount           = 10
	MaxFeaturedCount          = 50
	MaxURLLength              = 2048
)

type CreateCommunityRequest struct {
	Name            string   `json:"name"`
	Tagline         string   `json:"tagline"`
	Icon            string   `json:"icon"`
	ThemeColor      string   `json:"themeColor"`
	PrimaryLanguage string   `json:"primaryLanguage"`
	PrivacyMode     int      `json:"privacyMode"` // 0: Open, 1: Approval Required, 2: Private
	Keywords        []string `json:"keywords"`
}

func (r *CreateCommunityRequest) Validate() error {
	if len(r.Name) == 0 || len(r.Name) > MaxCommunityNameLength {
		return ErrValidation
	}
	if len(r.Tagline) > MaxCommunityTaglineLength {
		return ErrValidation
	}
	if len(r.Keywords) > MaxKeywordsCount {
		return ErrValidation
	}
	for _, kw := range r.Keywords {
		if len(kw) > MaxKeywordLength {
			return ErrValidation
		}
	}
	if r.PrivacyMode < 0 || r.PrivacyMode > 2 {
		return ErrValidation
	}
	return nil
}

type JoinCommunityRequest struct {
	InvitationCode string `json:"invitationId"` // Optional, for private comms
}

type BanRequest struct {
	Note struct {
		Content string `json:"content"`
	} `json:"note"`
}

func (r *BanRequest) Validate() error {
	if len(r.Note.Content) > MaxBanReasonLength {
		return ErrValidation
	}
	return nil
}

type ThemePackRequest struct {
	ThemeColor      *string `json:"themeColor,omitempty"`
	ThemeSideImage  *string `json:"themeSideImage,omitempty"`
	ThemeUpperImage *string `json:"themeUpperImage,omitempty"`
	Cover           *string `json:"cover,omitempty"`
}

type UpdateCommunitySettingsRequest struct {
	Name            *string           `json:"name"`
	Icon            *string           `json:"icon"`
	Content         *string           `json:"content"`
	Endpoint        *string           `json:"endpoint"`
	PrimaryLanguage *string           `json:"primaryLanguage"`
	ThemePack       *ThemePackRequest `json:"themePack"`
	Searchable      *bool             `json:"searchable"`
}

func (r *UpdateCommunitySettingsRequest) Validate() error {
	if r.Name != nil && (len(*r.Name) == 0 || len(*r.Name) > MaxCommunityNameLength) {
		return ErrValidation
	}
	if r.Content != nil && len(*r.Content) > MaxCommunityContentLength {
		return ErrValidation
	}
	if r.Endpoint != nil && len(*r.Endpoint) > MaxCommunityEndpointLength {
		return ErrValidation
	}
	if r.Icon != nil && len(*r.Icon) > MaxURLLength {
		return ErrValidation
	}
	return nil
}

// Featured Communities Requests

type SetFeaturedRequest struct {
	Segment string `json:"segment"` // ru, en, es, ar
	NdcIds  []int  `json:"ndcIds"`  // List of community NDC IDs in order
}

func (r *SetFeaturedRequest) Validate() error {
	if len(r.NdcIds) == 0 || len(r.NdcIds) > MaxFeaturedCount {
		return ErrValidation
	}
	return nil
}

type AddFeaturedRequest struct {
	Segment string `json:"segment"` // ru, en, es, ar
	NdcId   int    `json:"ndcId"`   // Single community NDC ID
}

type GetFeaturedByIdsRequest struct {
	NdcIds []int `json:"ndcIds"` // List of community NDC IDs
}

func (r *GetFeaturedByIdsRequest) Validate() error {
	if len(r.NdcIds) == 0 || len(r.NdcIds) > MaxFeaturedCount {
		return ErrValidation
	}
	return nil
}

// Banner Requests

type BannerItem struct {
	ImageURL string `json:"imageUrl"`
	LinkURL  string `json:"linkUrl"`
}

type SetBannersRequest struct {
	Segment string       `json:"segment"`
	Banners []BannerItem `json:"banners"`
}

func (r *SetBannersRequest) Validate() error {
	if len(r.Banners) > MaxBannersCount {
		return ErrValidation
	}
	for _, b := range r.Banners {
		if len(b.ImageURL) > MaxURLLength || len(b.LinkURL) > MaxURLLength {
			return ErrValidation
		}
	}
	return nil
}