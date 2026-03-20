package auth

import (
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
)

// LoginResponse - ответ на логин/регистрацию в формате Amino
type LoginResponse struct {
	AUID        string     `json:"auid"`
	Account     *Account   `json:"account"`
	Secret      string     `json:"secret"`
	SID         string     `json:"sid"`
	UserProfile *user.User `json:"userProfile"`
}

// Account - данные аккаунта (приватная информация)
type Account struct {
	Username              *string            `json:"username"`
	Status                int                `json:"status"`
	UID                   string             `json:"uid"`
	ModifiedTime          string             `json:"modifiedTime"`
	TwitterID             *string            `json:"twitterID"`
	Activation            int                `json:"activation"`
	PhoneNumberActivation int                `json:"phoneNumberActivation"`
	EmailActivation       int                `json:"emailActivation"`
	AppleID               *string            `json:"appleID"`
	FacebookID            *string            `json:"facebookID"`
	Nickname              string             `json:"nickname"`
	MediaList             []utils.MediaItem  `json:"mediaList"` // [[100, "url", null], ...]
	GoogleID              *string            `json:"googleID"`
	Icon                  string             `json:"icon"`
	SecurityLevel         int                `json:"securityLevel"`
	PhoneNumber           *string            `json:"phoneNumber"`
	Membership            *interface{}       `json:"membership"`
	AdvancedSettings      AdvancedSettings   `json:"advancedSettings"`
	Role                  int                `json:"role"`
	AminoIDEditable       bool               `json:"aminoIdEditable"`
	AminoID               string             `json:"aminoId"`
	CreatedTime           string             `json:"createdTime"`
	Extensions            *AccountExtensions `json:"extensions"`
	Email                 string             `json:"email"`
}

type AdvancedSettings struct {
	AnalyticsEnabled int `json:"analyticsEnabled"`
}

type AccountExtensions struct {
	ContentLanguage                string        `json:"contentLanguage"`
	AdsFlags                       int64         `json:"adsFlags"`
	AdsLevel                       int           `json:"adsLevel"`
	DeviceInfo                     DeviceInfo    `json:"deviceInfo"`
	MediaLabAdsMigrationJuly2020   bool          `json:"mediaLabAdsMigrationJuly2020"`
	VisitSettings                  VisitSettings `json:"visitSettings"`
	MediaLabAdsMigrationAugust2020 bool          `json:"mediaLabAdsMigrationAugust2020"`
	AdsEnabled                     bool          `json:"adsEnabled"`
}

type DeviceInfo struct {
	LastClientType int `json:"lastClientType"`
}

type VisitSettings struct {
	NotificationStatus int `json:"notificationStatus"`
	PrivacyMode        int `json:"privacyMode"`
}

// NewLoginResponse создаёт ответ для логина/регистрации
func NewLoginResponse(u *user.User, email, sid, secret, contentRegion string) *LoginResponse {
	timeFormat := "2006-01-02T15:04:05Z"

	// Default to "en" if content region is empty
	if contentRegion == "" {
		contentRegion = "en"
	}

	return &LoginResponse{
		AUID:   u.UID,
		Secret: secret,
		SID:    sid,
		Account: &Account{
			Username:              nil,
			Status:                u.Status,
			UID:                   u.UID,
			ModifiedTime:          u.ModifiedTime.Format(timeFormat),
			TwitterID:             nil,
			Activation:            1,
			PhoneNumberActivation: 0,
			EmailActivation:       1,
			AppleID:               nil,
			FacebookID:            nil,
			Nickname:              u.Nickname,
			MediaList:             nil,
			GoogleID:              nil,
			Icon:                  u.Icon,
			SecurityLevel:         3,
			PhoneNumber:           nil,
			Membership:            nil,
			AdvancedSettings:      AdvancedSettings{AnalyticsEnabled: 0},
			Role:                  u.Role,
			AminoIDEditable:       true,
			AminoID:               u.AminoID,
			CreatedTime:           u.CreatedTime.Format(timeFormat),
			Extensions: &AccountExtensions{
				ContentLanguage:                contentRegion,
				AdsFlags:                       2147483647,
				AdsLevel:                       2,
				DeviceInfo:                     DeviceInfo{LastClientType: 100},
				MediaLabAdsMigrationJuly2020:   true,
				VisitSettings:                  VisitSettings{NotificationStatus: 1, PrivacyMode: 1},
				MediaLabAdsMigrationAugust2020: true,
				AdsEnabled:                     true,
			},
			Email: email,
		},
		UserProfile: u,
	}
}
