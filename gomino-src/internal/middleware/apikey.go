package middleware

import (
	"strings"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

const (
	APIKeyContextKey    = "api_key"
	BypassSecurityKey   = "bypass_security"
)

// APIKeyMiddleware validates API keys and sets bypass flags
func APIKeyMiddleware(c fiber.Ctx) error {
	// Try to extract API key from headers
	apiKey := extractAPIKey(c)

	// API key is required for all requests
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusMissingAPIKey))
	}

	// Validate the API key
	db := GetDBFromContext(c)
	if db == nil {
		return c.Next()
	}

	apiKeyService := service.NewAPIKeyService(db)

	key, err := apiKeyService.ValidateAPIKey(apiKey)
	if err != nil {
		// Invalid API key - return error
		if err == service.ErrAPIKeyNotFound {
			return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusInvalidAPIKey))
		}
		if err == service.ErrAPIKeyExpired {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"api:statuscode": 403,
				"api:message":    "API key has expired",
			})
		}
		if err == service.ErrAPIKeyInactive {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"api:statuscode": 403,
				"api:message":    "API key is inactive",
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusInvalidAPIKey))
	}

	// Check rate limit
	if err := apiKeyService.RecordUsage(key); err != nil {
		if err == service.ErrAPIKeyRateLimited {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"api:statuscode": 429,
				"api:message":    "API key rate limit exceeded",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Valid API key - set context values
	c.Locals(APIKeyContextKey, key)

	// Only bootstrap keys (no UserID) bypass DPoP/Signature security
	if key.UserID == nil {
		c.Locals(BypassSecurityKey, true)
	}

	// If the API key has a user ID, set it as AUID for authentication
	if key.UserID != nil {
		c.Locals("user_id", *key.UserID)
	}

	return c.Next()
}

// extractAPIKey extracts API key from request headers
// Supports both X-API-Key and Authorization: Bearer headers
func extractAPIKey(c fiber.Ctx) string {
	// Try X-API-Key header first
	apiKey := c.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// Try Authorization: Bearer header
	auth := c.Get("Authorization")
	if auth != "" {
		parts := strings.Split(auth, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	return ""
}

// HasAPIKey checks if the request has a valid API key
func HasAPIKey(c fiber.Ctx) bool {
	return c.Locals(APIKeyContextKey) != nil
}

// ShouldBypassSecurity checks if security checks should be bypassed
func ShouldBypassSecurity(c fiber.Ctx) bool {
	bypass, ok := c.Locals(BypassSecurityKey).(bool)
	return ok && bypass
}

// GetAPIKeyFromContext retrieves the API key from context
func GetAPIKeyFromContext(c fiber.Ctx) interface{} {
	return c.Locals(APIKeyContextKey)
}
