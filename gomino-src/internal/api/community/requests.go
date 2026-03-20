package community

type CreateCommunityRequest struct {
	Name            string   `json:"name"`
	Tagline         string   `json:"tagline"`
	Icon            string   `json:"icon"`
	ThemeColor      string   `json:"themeColor"`
	PrimaryLanguage string   `json:"primaryLanguage"`
	PrivacyMode     int      `json:"privacyMode"` // 0: Open, 1: Approval Required, 2: Private
	Keywords        []string `json:"keywords"`
}

type JoinCommunityRequest struct {
	InvitationCode string `json:"invitationId"` // Optional, for private comms
}

type BanRequest struct {
	Note struct {
		Content string `json:"content"`
	} `json:"note"`
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

// Featured Communities Requests

type SetFeaturedRequest struct {
	Segment string `json:"segment"` // ru, en, es, ar
	NdcIds  []int  `json:"ndcIds"`  // List of community NDC IDs in order
}

type AddFeaturedRequest struct {
	Segment string `json:"segment"` // ru, en, es, ar
	NdcId   int    `json:"ndcId"`   // Single community NDC ID
}

type GetFeaturedByIdsRequest struct {
	NdcIds []int `json:"ndcIds"` // List of community NDC IDs
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