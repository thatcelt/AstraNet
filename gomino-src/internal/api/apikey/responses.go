package apikey

import (
	"encoding/json"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/apikey"
)

// CreateAPIKeyResponse - response after creating an API key
type CreateAPIKeyResponse struct {
	StatusCode int         `json:"api:statuscode"`
	Message    string      `json:"api:message"`
	APIKey     string      `json:"apiKey"`     // Plain text key (only shown once!)
	KeyInfo    APIKeyInfo  `json:"keyInfo"`    // Key metadata
}

// APIKeyInfo - metadata about an API key (without the actual key)
type APIKeyInfo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Scopes       []string   `json:"scopes"`
	RateLimit    int        `json:"rateLimit"`
	IsActive     bool       `json:"isActive"`
	CreatedAt    time.Time  `json:"createdAt"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt   *time.Time `json:"lastUsedAt,omitempty"`
	RequestCount int        `json:"requestCount"`
}

// ListAPIKeysResponse - response for listing API keys
type ListAPIKeysResponse struct {
	StatusCode int          `json:"api:statuscode"`
	APIKeys    []APIKeyInfo `json:"apiKeys"`
	Count      int          `json:"count"`
}

// APIKeyResponse - response for single API key
type APIKeyResponse struct {
	StatusCode int        `json:"api:statuscode"`
	APIKey     APIKeyInfo `json:"apiKey"`
}

func NewCreateAPIKeyResponse(plainKey string, key *apikey.APIKey) CreateAPIKeyResponse {
	return CreateAPIKeyResponse{
		StatusCode: 0,
		Message:    "API key created successfully. Save this key securely - it won't be shown again!",
		APIKey:     plainKey,
		KeyInfo:    convertToAPIKeyInfo(key),
	}
}

func NewListAPIKeysResponse(keys []apikey.APIKey) ListAPIKeysResponse {
	keyInfos := make([]APIKeyInfo, len(keys))
	for i, key := range keys {
		keyInfos[i] = convertToAPIKeyInfo(&key)
	}

	return ListAPIKeysResponse{
		StatusCode: 0,
		APIKeys:    keyInfos,
		Count:      len(keyInfos),
	}
}

func NewAPIKeyResponse(key *apikey.APIKey) APIKeyResponse {
	return APIKeyResponse{
		StatusCode: 0,
		APIKey:     convertToAPIKeyInfo(key),
	}
}

func convertToAPIKeyInfo(key *apikey.APIKey) APIKeyInfo {
	var scopes []string
	if key.Scopes != "" {
		json.Unmarshal([]byte(key.Scopes), &scopes)
	}
	if scopes == nil {
		scopes = []string{}
	}

	return APIKeyInfo{
		ID:           key.ID,
		Name:         key.Name,
		Scopes:       scopes,
		RateLimit:    key.RateLimit,
		IsActive:     key.IsActive,
		CreatedAt:    key.CreatedAt,
		ExpiresAt:    key.ExpiresAt,
		LastUsedAt:   key.LastUsedAt,
		RequestCount: key.RequestCount,
	}
}
