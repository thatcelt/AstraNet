package apikey

import (
	"log"
	"time"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// CreateAPIKey godoc
// @Summary Create a new API key
// @Description Create a new API key for the authenticated user
// @Tags apikey
// @Accept  json
// @Produce  json
// @Param   request body CreateAPIKeyRequest true "API key creation data"
// @Success 200 {object} CreateAPIKeyResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/developer/api-keys [post]
func CreateAPIKey(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)

	var req CreateAPIKeyRequest
	if err := c.Bind().Body(&req); err != nil {
		log.Printf("Failed to parse create API key request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	if err := req.Validate(); err != nil {
		log.Printf("Create API key validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	// Get authenticated user ID
	userID := c.Get("AUID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	apiKeyService := service.NewAPIKeyService(db)

	// Set default rate limit if not provided
	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = 1000 // Default: 1000 requests per hour
	}

	// Parse expiration duration if provided
	var expiresIn *time.Duration
	if req.ExpiresInDays > 0 {
		duration := time.Duration(req.ExpiresInDays) * 24 * time.Hour
		expiresIn = &duration
	}

	// Create API key
	plainKey, key, err := apiKeyService.CreateAPIKey(&userID, req.Name, req.Scopes, rateLimit, expiresIn)
	if err != nil {
		log.Printf("Failed to create API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(NewCreateAPIKeyResponse(plainKey, key))
}

// ListAPIKeys godoc
// @Summary List all API keys
// @Description List all API keys for the authenticated user
// @Tags apikey
// @Accept  json
// @Produce  json
// @Success 200 {object} ListAPIKeysResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/developer/api-keys [get]
func ListAPIKeys(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)

	// Get authenticated user ID
	userID := c.Get("AUID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	apiKeyService := service.NewAPIKeyService(db)

	keys, err := apiKeyService.ListAPIKeys(userID)
	if err != nil {
		log.Printf("Failed to list API keys: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(NewListAPIKeysResponse(keys))
}

// GetAPIKey godoc
// @Summary Get API key details
// @Description Get details for a specific API key
// @Tags apikey
// @Accept  json
// @Produce  json
// @Param   keyId path string true "API Key ID"
// @Success 200 {object} APIKeyResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/developer/api-keys/{keyId} [get]
func GetAPIKey(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	keyID := c.Params("keyId")

	// Get authenticated user ID
	userID := c.Get("AUID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	apiKeyService := service.NewAPIKeyService(db)

	key, err := apiKeyService.GetAPIKey(keyID)
	if err != nil {
		if err == service.ErrAPIKeyNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(404, "API key not found"))
		}
		log.Printf("Failed to get API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Check ownership
	if key.UserID == nil || *key.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
	}

	return c.Status(fiber.StatusOK).JSON(NewAPIKeyResponse(key))
}

// RevokeAPIKey godoc
// @Summary Revoke an API key
// @Description Revoke (delete) an API key
// @Tags apikey
// @Accept  json
// @Produce  json
// @Param   keyId path string true "API Key ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/developer/api-keys/{keyId} [delete]
func RevokeAPIKey(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	keyID := c.Params("keyId")

	// Get authenticated user ID
	userID := c.Get("AUID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	apiKeyService := service.NewAPIKeyService(db)

	if err := apiKeyService.RevokeAPIKey(keyID, &userID); err != nil {
		if err == service.ErrAPIKeyNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(404, "API key not found"))
		}
		if err == service.ErrAPIKeyUnauthorized {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		log.Printf("Failed to revoke API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"api:statuscode": 0,
		"api:message":    "API key revoked successfully",
	})
}

// UpdateAPIKey godoc
// @Summary Update an API key
// @Description Update properties of an API key
// @Tags apikey
// @Accept  json
// @Produce  json
// @Param   keyId path string true "API Key ID"
// @Param   request body UpdateAPIKeyRequest true "API key update data"
// @Success 200 {object} APIKeyResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/developer/api-keys/{keyId} [patch]
func UpdateAPIKey(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	keyID := c.Params("keyId")

	var req UpdateAPIKeyRequest
	if err := c.Bind().Body(&req); err != nil {
		log.Printf("Failed to parse update API key request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	// Get authenticated user ID
	userID := c.Get("AUID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	apiKeyService := service.NewAPIKeyService(db)

	// Build updates map
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.RateLimit != nil {
		updates["rate_limit"] = *req.RateLimit
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := apiKeyService.UpdateAPIKey(keyID, &userID, updates); err != nil {
		if err == service.ErrAPIKeyNotFound {
			return c.Status(fiber.StatusNotFound).JSON(response.NewErrorWithMessage(404, "API key not found"))
		}
		if err == service.ErrAPIKeyUnauthorized {
			return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusNoPermission))
		}
		log.Printf("Failed to update API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Fetch updated key
	key, err := apiKeyService.GetAPIKey(keyID)
	if err != nil {
		log.Printf("Failed to fetch updated API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(NewAPIKeyResponse(key))
}
