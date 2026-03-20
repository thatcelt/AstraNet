package auth

import (
	"errors"
	"strings"
)

var ErrValidation = errors.New("validation failed")

// LoginRequest - запрос на логин
type LoginRequest struct {
	Email         string `json:"email"`
	Secret        string `json:"secret"`     // формат: "0 $password"
	ClientType    int    `json:"clientType"` // должно быть 100
	DeviceID      string `json:"deviceID"`
	V             int    `json:"v"` // версия, должно быть 2
	DPoPPublicKey string `json:"dpopPublicKey,omitempty"` // DPoP public key for token binding
	DPoPKeyID     string `json:"dpopKeyId,omitempty"`     // DPoP key ID
}

func (r *LoginRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || !strings.Contains(r.Email, "@") {
		return ErrValidation
	}
	if r.Secret == "" || r.ClientType != 100 || r.V != 2 {
		return ErrValidation
	}
	if r.DeviceID == "" {
		return ErrValidation
	}
	return nil
}

// RegisterRequest - запрос на регистрацию
type RegisterRequest struct {
	Secret            string            `json:"secret"`
	DeviceID          string            `json:"deviceID"`
	Email             string            `json:"email"`
	ClientType        int               `json:"clientType"`
	Nickname          string            `json:"nickname"`
	Latitude          float64           `json:"latitude"`
	Longitude         float64           `json:"longitude"`
	Address           *string           `json:"address"`
	ClientCallbackURL string            `json:"clientCallbackURL"`
	ValidationContext ValidationContext `json:"validationContext"`
	Type              int               `json:"type"`
	Identity          string            `json:"identity"`
	DPoPPublicKey     string            `json:"dpopPublicKey,omitempty"` // DPoP public key for token binding
	DPoPKeyID         string            `json:"dpopKeyId,omitempty"`     // DPoP key ID
}

type ValidationContext struct {
	Data     ValidationData `json:"data"`
	Type     int            `json:"type"`
	Level    *int           `json:"level,omitempty"`
	Identity string         `json:"identity"`
}

type ValidationData struct {
	Code string `json:"code"`
}

func (r *RegisterRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	r.Identity = strings.TrimSpace(r.Identity)
	if r.Email == "" || !strings.Contains(r.Email, "@") {
		return ErrValidation
	}
	if r.Identity == "" || r.Identity != r.Email {
		return ErrValidation
	}
	if r.Nickname == "" || len(r.Nickname) < 3 {
		return ErrValidation
	}
	if r.Secret == "" || r.DeviceID == "" {
		return ErrValidation
	}
	if r.ClientType != 100 || r.Type != 1 {
		return ErrValidation
	}
	if r.ValidationContext.Type != 1 {
		return ErrValidation
	}
	if r.ValidationContext.Identity != r.Email {
		return ErrValidation
	}
	if r.ValidationContext.Data.Code == "" {
		return ErrValidation
	}
	return nil
}

// RefreshTokenRequest - запрос на обновление токена
type RefreshTokenRequest struct {
	Secret        string `json:"secret"`                  // amino secret used as refresh token
	DeviceID      string `json:"deviceID"`
	DPoPPublicKey string `json:"dpopPublicKey,omitempty"` // DPoP public key for token binding
	DPoPKeyID     string `json:"dpopKeyId,omitempty"`     // DPoP key ID
}

func (r *RefreshTokenRequest) Validate() error {
	if r.Secret == "" {
		return ErrValidation
	}
	if r.DeviceID == "" {
		return ErrValidation
	}
	return nil
}

// SecurityValidationRequest - запрос на отправку кода верификации
type SecurityValidationRequest struct {
	DeviceID string `json:"deviceID"`
	Type     int    `json:"type"`
	Identity string `json:"identity"` // email
}

func (r *SecurityValidationRequest) Validate() error {
	r.Identity = strings.TrimSpace(r.Identity)
	if r.DeviceID == "" {
		return ErrValidation
	}
	if r.Identity == "" || !strings.Contains(r.Identity, "@") {
		return ErrValidation
	}
	if r.Type != 1 {
		return ErrValidation
	}
	return nil
}
