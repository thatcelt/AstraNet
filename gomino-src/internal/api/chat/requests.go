package chat

// CreateThreadRequest - запрос на создание чата
type CreateThreadRequest struct {
	Title       string   `json:"title" binding:"required,min=1,max=500"`
	Content     string   `json:"content" binding:"max=5000"`
	Type        int      `json:"type" binding:"required,min=0,max=2"`
	Icon        string   `json:"icon,omitempty"`
	InviteeUids []string `json:"inviteeUids,omitempty"`
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

// EditMessageRequest - запрос на редактирование сообщения
type EditMessageRequest struct {
	Content string `json:"content"`
}

// InviteRequest - запрос на приглашение пользователей
type InviteRequest struct {
	Uids []string `json:"uids"`
}
