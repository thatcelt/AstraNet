package auth

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var (
	ErrValidation       = errors.New("validation failed")
	ErrInvalidEmail     = errors.New("invalid email")
	ErrPasswordTooShort = errors.New("password too short")
	ErrPasswordTooLong  = errors.New("password too long")
	ErrInvalidSecret    = errors.New("invalid secret format")
	ErrNicknameTooShort = errors.New("nickname too short")
	ErrNicknameTooLong  = errors.New("nickname too long")
	ErrNicknameEmpty    = errors.New("nickname empty")
	ErrInvalidDeviceID  = errors.New("invalid device id")
	ErrInvalidIdentity  = errors.New("identity mismatch")
	ErrInvalidCode      = errors.New("invalid verification code")
)

// emailRegex validates email format (RFC 5322 simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

// validatePasswordStrength checks password meets minimum requirements
func validatePasswordStrength(password string) bool {
	if len(password) < 8 || len(password) > 128 {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

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
	r.Email = strings.ToLower(r.Email)
	if r.Email == "" || !isValidEmail(r.Email) {
		return ErrValidation
	}
	if r.Secret == "" || r.ClientType != 100 || r.V != 2 {
		return ErrValidation
	}
	if r.DeviceID == "" || len(r.DeviceID) > 128 {
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
	r.Email = strings.ToLower(r.Email)
	r.Identity = strings.TrimSpace(r.Identity)
	r.Identity = strings.ToLower(r.Identity)
	if r.Email == "" || !isValidEmail(r.Email) {
		return ErrInvalidEmail
	}
	if r.Identity == "" || r.Identity != r.Email {
		return ErrInvalidIdentity
	}
	r.Nickname = strings.TrimSpace(r.Nickname)
	if r.Nickname == "" {
		return ErrNicknameEmpty
	}
	if len(r.Nickname) < 3 {
		return ErrNicknameTooShort
	}
	if len(r.Nickname) > 32 {
		return ErrNicknameTooLong
	}
	if r.Secret == "" {
		return ErrInvalidSecret
	}
	if r.DeviceID == "" || len(r.DeviceID) > 128 {
		return ErrInvalidDeviceID
	}
	password := strings.TrimPrefix(r.Secret, "0 ")
	if password == r.Secret {
		return ErrInvalidSecret
	}
	if len(password) < 6 {
		return ErrPasswordTooShort
	}
	if len(password) > 128 {
		return ErrPasswordTooLong
	}
	if r.ClientType != 100 || r.Type != 1 {
		return ErrValidation
	}
	if r.ValidationContext.Type != 1 {
		return ErrValidation
	}
	if r.ValidationContext.Identity != r.Email {
		return ErrInvalidIdentity
	}
	if r.ValidationContext.Data.Code == "" || len(r.ValidationContext.Data.Code) > 10 {
		return ErrInvalidCode
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
	r.Identity = strings.ToLower(r.Identity)
	if r.DeviceID == "" || len(r.DeviceID) > 128 {
		return ErrValidation
	}
	if r.Identity == "" || !isValidEmail(r.Identity) {
		return ErrValidation
	}
	if r.Type != 1 {
		return ErrValidation
	}
	return nil
}
