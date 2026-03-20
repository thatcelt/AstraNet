package service

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/AugustLigh/GoMino/internal/models/apikey"
	"gorm.io/gorm"
)

var (
	ErrAPIKeyNotFound      = errors.New("API key not found")
	ErrAPIKeyInvalid       = errors.New("API key is invalid")
	ErrAPIKeyExpired       = errors.New("API key has expired")
	ErrAPIKeyInactive      = errors.New("API key is inactive")
	ErrAPIKeyRateLimited   = errors.New("API key rate limit exceeded")
	ErrAPIKeyUnauthorized  = errors.New("unauthorized to manage API keys")
)

type APIKeyService struct {
	db *gorm.DB
}

func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// CreateAPIKey creates a new API key
// Returns the plain text key (only shown once) and the API key object
func (s *APIKeyService) CreateAPIKey(userID *string, name string, scopes []string, rateLimit int, expiresIn *time.Duration) (string, *apikey.APIKey, error) {
	// Generate the key
	plainKey, keyHash, err := apikey.GenerateAPIKey()
	if err != nil {
		return "", nil, err
	}

	// Marshal scopes to JSON
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return "", nil, err
	}

	// Create API key object
	key := &apikey.APIKey{
		KeyHash:   keyHash,
		UserID:    userID,
		Name:      name,
		Scopes:    string(scopesJSON),
		RateLimit: rateLimit,
		IsActive:  true,
	}

	// Set expiration if provided
	if expiresIn != nil {
		expiresAt := time.Now().Add(*expiresIn)
		key.ExpiresAt = &expiresAt
	}

	// Save to database
	if err := s.db.Create(key).Error; err != nil {
		return "", nil, err
	}

	return plainKey, key, nil
}

// ValidateAPIKey validates a key and returns the API key object if valid
func (s *APIKeyService) ValidateAPIKey(plainKey string) (*apikey.APIKey, error) {
	// Find all active keys (we need to hash-compare)
	var keys []apikey.APIKey
	if err := s.db.Where("is_active = ?", true).Find(&keys).Error; err != nil {
		return nil, err
	}

	// Check each key's hash
	for _, key := range keys {
		if apikey.ValidateAPIKey(plainKey, key.KeyHash) {
			// Found matching key
			if key.IsExpired() {
				return nil, ErrAPIKeyExpired
			}

			if !key.IsActive {
				return nil, ErrAPIKeyInactive
			}

			return &key, nil
		}
	}

	return nil, ErrAPIKeyNotFound
}

// CheckRateLimit checks if the API key has exceeded its rate limit
func (s *APIKeyService) CheckRateLimit(keyID string, rateLimit int) error {
	// Get current hourly window
	now := time.Now()
	window := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// Find or create usage record for this window
	var usage apikey.APIKeyUsage
	err := s.db.Where("key_id = ? AND window = ?", keyID, window).First(&usage).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new usage record
			usage = apikey.APIKeyUsage{
				KeyID:  keyID,
				Window: window,
				Count:  1,
			}
			return s.db.Create(&usage).Error
		}
		return err
	}

	// Check if rate limit exceeded
	if usage.Count >= rateLimit {
		return ErrAPIKeyRateLimited
	}

	// Increment count
	usage.Count++
	return s.db.Save(&usage).Error
}

// RecordUsage records API key usage and checks rate limit
func (s *APIKeyService) RecordUsage(key *apikey.APIKey) error {
	// Check rate limit first
	if err := s.CheckRateLimit(key.ID, key.RateLimit); err != nil {
		return err
	}

	// Update last used timestamp
	return key.UpdateLastUsed(s.db)
}

// GetAPIKey retrieves an API key by ID
func (s *APIKeyService) GetAPIKey(keyID string) (*apikey.APIKey, error) {
	var key apikey.APIKey
	if err := s.db.Where("id = ?", keyID).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return &key, nil
}

// ListAPIKeys lists all API keys for a user
func (s *APIKeyService) ListAPIKeys(userID string) ([]apikey.APIKey, error) {
	var keys []apikey.APIKey
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// ListAllAPIKeys lists all API keys (admin only)
func (s *APIKeyService) ListAllAPIKeys() ([]apikey.APIKey, error) {
	var keys []apikey.APIKey
	if err := s.db.Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// RevokeAPIKey revokes (soft deletes) an API key
func (s *APIKeyService) RevokeAPIKey(keyID string, userID *string) error {
	key, err := s.GetAPIKey(keyID)
	if err != nil {
		return err
	}

	// Check ownership if userID provided (non-admin)
	if userID != nil && (key.UserID == nil || *key.UserID != *userID) {
		return ErrAPIKeyUnauthorized
	}

	// Soft delete
	return s.db.Delete(&apikey.APIKey{}, "id = ?", keyID).Error
}

// DeactivateAPIKey deactivates an API key without deleting it
func (s *APIKeyService) DeactivateAPIKey(keyID string, userID *string) error {
	key, err := s.GetAPIKey(keyID)
	if err != nil {
		return err
	}

	// Check ownership if userID provided (non-admin)
	if userID != nil && (key.UserID == nil || *key.UserID != *userID) {
		return ErrAPIKeyUnauthorized
	}

	key.IsActive = false
	return s.db.Save(key).Error
}

// ActivateAPIKey reactivates a deactivated API key
func (s *APIKeyService) ActivateAPIKey(keyID string, userID *string) error {
	key, err := s.GetAPIKey(keyID)
	if err != nil {
		return err
	}

	// Check ownership if userID provided (non-admin)
	if userID != nil && (key.UserID == nil || *key.UserID != *userID) {
		return ErrAPIKeyUnauthorized
	}

	key.IsActive = true
	return s.db.Save(key).Error
}

// UpdateAPIKey updates API key properties
func (s *APIKeyService) UpdateAPIKey(keyID string, userID *string, updates map[string]interface{}) error {
	key, err := s.GetAPIKey(keyID)
	if err != nil {
		return err
	}

	// Check ownership if userID provided (non-admin)
	if userID != nil && (key.UserID == nil || *key.UserID != *userID) {
		return ErrAPIKeyUnauthorized
	}

	// Don't allow updating key hash
	delete(updates, "key_hash")
	delete(updates, "id")

	return s.db.Model(key).Updates(updates).Error
}

// GetAPIKeyScopes returns the scopes for an API key as a string slice
func (s *APIKeyService) GetAPIKeyScopes(key *apikey.APIKey) ([]string, error) {
	var scopes []string
	if err := json.Unmarshal([]byte(key.Scopes), &scopes); err != nil {
		return nil, err
	}
	return scopes, nil
}

// CleanupOldUsageRecords removes usage records older than 24 hours (housekeeping)
func (s *APIKeyService) CleanupOldUsageRecords() error {
	cutoff := time.Now().Add(-24 * time.Hour)
	return s.db.Where("window < ?", cutoff).Delete(&apikey.APIKeyUsage{}).Error
}
