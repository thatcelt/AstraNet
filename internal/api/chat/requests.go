package chat

import "errors"

var ErrValidation = errors.New("validation failed")

const (
	MaxMessageLength     = 2000
	MaxThreadTitleLength = 100
	MaxThreadContentLength = 500
	MaxAnnouncementLength  = 500
	MaxKeywordsLength      = 200
	MaxInvitees            = 50
)

// CreateThreadRequest - запрос на создание чата
type CreateThreadRequest struct {
	Title       string   `json:"title" binding:"required,min=1,max=500"`
	Content     string   `json:"content" binding:"max=5000"`
	Type        int      `json:"type" binding:"required,min=0,max=2"`
	Icon        string   `json:"icon,omitempty"`
	InviteeUids []string `json:"inviteeUids,omitempty"`
}

func (r *CreateThreadRequest) Validate() error {
	if len(r.Title) > MaxThreadTitleLength {
		return ErrValidation
	}
	if len(r.Content) > MaxThreadContentLength {
		return ErrValidation
	}
	if len(r.InviteeUids) > MaxInvitees {
		return ErrValidation
	}
	if r.Type < 0 || r.Type > 2 {
		return ErrValidation
	}
	return nil
}

// SendMessageRequest - запрос на отправку сообщения
type SendMessageRequest struct {
	Content        string                 `json:"content"`
	Type           int                    `json:"type"`
	MediaType      int                    `json:"mediaType,omitempty"`
	MediaValue     string                 `json:"mediaValue,omitempty"`
	StickerId      string                 `json:"stickerId,omitempty"`
	ReplyMessageId string                 `json:"replyMessageId,omitempty"`
	ClientRefId    int                    `json:"clientRefId,omitempty"`
	Extensions     map[string]interface{} `json:"extensions,omitempty"`
}

func (r *SendMessageRequest) Validate() error {
	if len(r.Content) > MaxMessageLength {
		return ErrValidation
	}
	if len(r.MediaValue) > 2048 {
		return ErrValidation
	}
	return nil
}

// UpdateThreadRequest - запрос на обновление чата
type UpdateThreadRequest struct {
	Title           *string `json:"title,omitempty"`
	Content         *string `json:"content,omitempty"`
	Icon            *string `json:"icon,omitempty"`
	Keywords        *string `json:"keywords,omitempty"`
	PublishToGlobal *int    `json:"publishToGlobal,omitempty"`
	Extensions      struct {
		Announcement     *string       `json:"announcement,omitempty"`
		PinAnnouncement  *bool         `json:"pinAnnouncement,omitempty"`
		FansOnly         *bool         `json:"fansOnly,omitempty"`
		Language         *string       `json:"language,omitempty"`
		MembersCanInvite *bool         `json:"membersCanInvite,omitempty"`
		BM               []interface{} `json:"bm,omitempty"`
	} `json:"extensions,omitempty"`
}

func (r *UpdateThreadRequest) Validate() error {
	if r.Title != nil && len(*r.Title) > MaxThreadTitleLength {
		return ErrValidation
	}
	if r.Content != nil && len(*r.Content) > MaxThreadContentLength {
		return ErrValidation
	}
	if r.Keywords != nil && len(*r.Keywords) > MaxKeywordsLength {
		return ErrValidation
	}
	if r.Extensions.Announcement != nil && len(*r.Extensions.Announcement) > MaxAnnouncementLength {
		return ErrValidation
	}
	return nil
}

// EditMessageRequest - запрос на редактирование сообщения
type EditMessageRequest struct {
	Content string `json:"content"`
}

func (r *EditMessageRequest) Validate() error {
	if len(r.Content) > MaxMessageLength {
		return ErrValidation
	}
	return nil
}

// InviteRequest - запрос на приглашение пользователей
type InviteRequest struct {
	Uids []string `json:"uids"`
}

func (r *InviteRequest) Validate() error {
	if len(r.Uids) > MaxInvitees || len(r.Uids) == 0 {
		return ErrValidation
	}
	return nil
}
