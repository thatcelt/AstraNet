package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// APIKey represents a developer API key for bypassing security checks
type APIKey struct {
	ID          string         `json:"id" gorm:"primaryKey"`
	KeyHash     string         `json:"-" gorm:"uniqueIndex;not null"` // SHA256 hash of the key (never exposed)
	UserID      *string        `json:"userId,omitempty" gorm:"index"`  // Owner of the key (optional)
	Name        string         `json:"name" gorm:"not null"`           // Friendly name ("My Python Bot")
	Scopes      string         `json:"scopes" gorm:"type:json"`        // JSON array of permissions
	RateLimit   int            `json:"rateLimit" gorm:"default:1000"`  // Requests per hour
	IsActive    bool           `json:"isActive" gorm:"default:true"`
	CreatedAt   time.Time      `json:"createdAt"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	LastUsedAt  *time.Time     `json:"lastUsedAt,omitempty"`
	RequestCount int           `json:"requestCount" gorm:"default:0"` // Total requests made
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// APIKeyUsage tracks rate limiting for API keys
type APIKeyUsage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	KeyID     string    `json:"keyId" gorm:"index:idx_key_window;not null"`
	Window    time.Time `json:"window" gorm:"index:idx_key_window;not null"` // Hourly window
	Count     int       `json:"count" gorm:"default:0"`
	CreatedAt time.Time `json:"createdAt"`
}

const (
	APIKeyLength = 32 // bytes (64 hex characters)
	APIKeyPrefix = "gm_" // Prefix for easy identification
)

// GenerateAPIKey creates a new cryptographically secure API key
func GenerateAPIKey() (string, string, error) {
	// Generate random bytes
	keyBytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", err
	}

	// Create hex representation with prefix
	key := APIKeyPrefix + hex.EncodeToString(keyBytes)

	// Create SHA256 hash for storage
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	return key, keyHash, nil
}

// ValidateAPIKey checks if a key matches the stored hash
func ValidateAPIKey(key, storedHash string) bool {
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])
	return keyHash == storedHash
}

// BeforeCreate hook for GORM
func (k *APIKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		// Generate UUID-like ID
		idBytes := make([]byte, 16)
		if _, err := rand.Read(idBytes); err != nil {
			return err
		}
		k.ID = hex.EncodeToString(idBytes)
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}
	return nil
}

// IsExpired checks if the key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid checks if the key is active and not expired
func (k *APIKey) IsValid() bool {
	return k.IsActive && !k.IsExpired()
}

// UpdateLastUsed updates the last used timestamp
func (k *APIKey) UpdateLastUsed(db *gorm.DB) error {
	now := time.Now()
	k.LastUsedAt = &now
	k.RequestCount++
	return db.Model(k).Updates(map[string]interface{}{
		"last_used_at":  now,
		"request_count": gorm.Expr("request_count + 1"),
	}).Error
}
