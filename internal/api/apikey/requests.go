package apikey

import "errors"

var ErrValidation = errors.New("validation failed")

// CreateAPIKeyRequest - request to create a new API key
type CreateAPIKeyRequest struct {
	Name          string   `json:"name"`                    // Friendly name for the key
	Scopes        []string `json:"scopes,omitempty"`        // Permissions (future use)
	RateLimit     int      `json:"rateLimit,omitempty"`     // Requests per hour (default: 1000)
	ExpiresInDays int      `json:"expiresInDays,omitempty"` // Expiration in days (0 = no expiration)
}

func (r *CreateAPIKeyRequest) Validate() error {
	if r.Name == "" {
		return ErrValidation
	}
	if len(r.Name) < 3 || len(r.Name) > 255 {
		return ErrValidation
	}
	if r.RateLimit < 0 {
		return ErrValidation
	}
	if r.ExpiresInDays < 0 {
		return ErrValidation
	}
	return nil
}

// UpdateAPIKeyRequest - request to update an API key
type UpdateAPIKeyRequest struct {
	Name      *string `json:"name,omitempty"`
	RateLimit *int    `json:"rateLimit,omitempty"`
	IsActive  *bool   `json:"isActive,omitempty"`
}
