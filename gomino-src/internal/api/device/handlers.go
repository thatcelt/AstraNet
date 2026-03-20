package device

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/AugustLigh/GoMino/internal/models"
)

type DeviceTokenRequest struct {
	Token    string `json:"token" validate:"required"`
	Platform string `json:"platform" validate:"required,oneof=android ios web"`
}

// RegisterDeviceToken registers or updates a device token for push notifications
// @Summary Register device token
// @Description Register or update a Firebase Cloud Messaging token for the authenticated user
// @Tags Device
// @Accept json
// @Produce json
// @Param body body DeviceTokenRequest true "Device token data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /device-token [post]
func RegisterDeviceToken(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get user ID from context (set by auth middleware)
		userID := c.Locals("uid")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"api:statuscode": 401,
				"api:message":    "Unauthorized",
			})
		}

		uid := userID.(string)

		var req DeviceTokenRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"api:statuscode": 400,
				"api:message":    "Invalid request body",
			})
		}

		// Validate platform
		if req.Platform != models.PlatformAndroid &&
			req.Platform != models.PlatformIOS &&
			req.Platform != models.PlatformWeb {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"api:statuscode": 400,
				"api:message":    "Invalid platform. Must be android, ios, or web",
			})
		}

		// Check if token already exists
		var existingToken models.DeviceToken
		err := db.Where("token = ?", req.Token).First(&existingToken).Error

		if err == nil {
			// Token exists - update user_id and platform if needed
			if existingToken.UserID != uid || existingToken.Platform != req.Platform {
				existingToken.UserID = uid
				existingToken.Platform = req.Platform
				existingToken.UpdatedAt = time.Now()

				if err := db.Save(&existingToken).Error; err != nil {
					log.Printf("Error updating device token: %v", err)
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"api:statuscode": 500,
						"api:message":    "Failed to update device token",
					})
				}
			}

			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"api:statuscode": 200,
				"api:message":    "Device token updated successfully",
				"deviceToken":    existingToken,
			})
		}

		// Token doesn't exist - create new one
		if err != gorm.ErrRecordNotFound {
			log.Printf("Error checking device token: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"api:statuscode": 500,
				"api:message":    "Database error",
			})
		}

		newToken := models.DeviceToken{
			UserID:    uid,
			Token:     req.Token,
			Platform:  req.Platform,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := db.Create(&newToken).Error; err != nil {
			log.Printf("Error creating device token: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"api:statuscode": 500,
				"api:message":    "Failed to register device token",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"api:statuscode": 200,
			"api:message":    "Device token registered successfully",
			"deviceToken":    newToken,
		})
	}
}

// DeleteDeviceToken deletes all device tokens for the authenticated user
// @Summary Delete device tokens
// @Description Delete all device tokens for the authenticated user (called on logout)
// @Tags Device
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /device-token [delete]
func DeleteDeviceToken(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get user ID from context
		userID := c.Locals("uid")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"api:statuscode": 401,
				"api:message":    "Unauthorized",
			})
		}

		uid := userID.(string)

		// Delete all tokens for this user
		result := db.Where("user_id = ?", uid).Delete(&models.DeviceToken{})
		if result.Error != nil {
			log.Printf("Error deleting device tokens: %v", result.Error)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"api:statuscode": 500,
				"api:message":    "Failed to delete device tokens",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"api:statuscode": 200,
			"api:message":    "Device tokens deleted successfully",
			"deletedCount":   result.RowsAffected,
		})
	}
}

// GetUserDeviceTokens returns all device tokens for a specific user
// Internal helper function for sending push notifications
func GetUserDeviceTokens(db *gorm.DB, userID string) ([]models.DeviceToken, error) {
	var tokens []models.DeviceToken
	err := db.Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}
