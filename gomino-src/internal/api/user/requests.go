package user

import (
	usermodel "github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
)

// ExtensionsRequest - структура для extensions в запросе
type StyleRequest struct {
	BackgroundMediaList []utils.MediaItem `json:"backgroundMediaList,omitempty"`
	BackgroundColor     *string           `json:"backgroundColor,omitempty"`
}

type ExtensionsRequest struct {
	Style                        *StyleRequest `json:"style,omitempty"`
	DefaultBubbleId              *string       `json:"defaultBubbleId,omitempty"`
	PrivilegeOfChatInviteRequest *string       `json:"privilegeOfChatInviteRequest,omitempty"`
	CustomTitles                 []CustomTitleRequest `json:"customTitles,omitempty"`
}

type CustomTitleRequest struct {
	Title string `json:"title"`
	Color string `json:"color"`
}

// UpdateUserRequest - запрос на обновление профиля
type UpdateUserRequest struct {
	Nickname                       *string           `json:"nickname,omitempty"`
	Mood                           *string           `json:"mood,omitempty"`
	MoodSticker                    *string           `json:"moodSticker,omitempty"`
	AvatarFrameID                  *string           `json:"avatarFrameId,omitempty"`
	NotificationSubscriptionStatus *int              `json:"notificationSubscriptionStatus,omitempty"`
	PushEnabled                    *bool             `json:"pushEnabled,omitempty"`
	Content                        *string           `json:"content,omitempty"`
	MediaList                      utils.MediaList `json:"mediaList,omitempty"`
	Icon                           *string           `json:"icon,omitempty"`
	Extensions                     *ExtensionsRequest `json:"extensions,omitempty"`
}

func (r *UpdateUserRequest) Validate() error {
	if r.Nickname != nil && len(*r.Nickname) > 30 {
		return ErrValidation
	}
	return nil
}

// ToMap конвертирует в map для GORM Updates
func (r *UpdateUserRequest) ToMap() map[string]interface{} {
	updates := make(map[string]interface{})

	if r.Nickname != nil {
		updates["nickname"] = *r.Nickname
	}
	if r.Mood != nil {
		updates["mood"] = *r.Mood
	}
	if r.MoodSticker != nil {
		updates["mood_sticker"] = *r.MoodSticker
	}
	if r.AvatarFrameID != nil {
		updates["avatar_frame_id"] = *r.AvatarFrameID
	}
	if r.NotificationSubscriptionStatus != nil {
		updates["notification_subscription_status"] = *r.NotificationSubscriptionStatus
	}
	if r.PushEnabled != nil {
		updates["push_enabled"] = *r.PushEnabled
	}
	if r.Content != nil {
		updates["content"] = *r.Content
	}
	if len(r.MediaList) > 0 {
		updates["media_list"] = r.MediaList
	}
	if r.Icon != nil {
		updates["icon"] = *r.Icon
	}
	if r.Extensions != nil {
		updates["extensions"] = r.ExtensionsToModel()
	}

	return updates
}

// ExtensionsToModel конвертирует ExtensionsRequest в модель Extensions
func (r *UpdateUserRequest) ExtensionsToModel() usermodel.Extensions {
	ext := usermodel.Extensions{}

	if r.Extensions.Style != nil {
		ext.Style.BackgroundMediaList = make([]usermodel.BackgroundMedia, len(r.Extensions.Style.BackgroundMediaList))
		for i, item := range r.Extensions.Style.BackgroundMediaList {
			ext.Style.BackgroundMediaList[i] = usermodel.BackgroundMedia{item.Type, item.URL, item.Extra1}
		}
	}

	return ext
}

// CreateCommentRequest - запрос на создание комментария
type CreateCommentRequest struct {
	Content   string  `json:"content"`
	StickerID *string `json:"stickerId"`
	Type      int     `json:"type"`
	RespondTo *string `json:"respondTo,omitempty"`
}
