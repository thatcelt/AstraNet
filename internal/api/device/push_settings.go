package device

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/AugustLigh/GoMino/internal/models"
	"github.com/AugustLigh/GoMino/internal/models/user"
)

type PushSettingsResponse struct {
	Enabled bool `json:"enabled"`
}

type UpdatePushSettingsRequest struct {
	Enabled bool `json:"enabled"`
}

// GetPushSettings returns the current push notification settings for the authenticated user
// @Summary Get push notification settings
// @Description Get the current push notification settings (enabled/disabled)
// @Tags Device
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /account/push-settings [get]
func GetPushSettings(db *gorm.DB) fiber.Handler {
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

		// Get user push settings
		var u user.User
		if err := db.Select("push_enabled").Where("uid = ? AND ndc_id = 0", uid).First(&u).Error; err != nil {
			log.Printf("Error getting user push settings: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"api:statuscode": 500,
				"api:message":    "Failed to get push settings",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"api:statuscode": 200,
			"enabled":        u.PushEnabled,
		})
	}
}

// UpdatePushSettings enables or disables push notifications for the authenticated user
// @Summary Update push notification settings
// @Description Enable or disable push notifications. When disabled, all device tokens are deleted.
// @Tags Device
// @Accept json
// @Produce json
// @Param body body UpdatePushSettingsRequest true "Push settings"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /account/push-settings [post]
func UpdatePushSettings(db *gorm.DB) fiber.Handler {
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

		var req UpdatePushSettingsRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"api:statuscode": 400,
				"api:message":    "Invalid request body",
			})
		}

		// Update user push_enabled field in GLOBAL profile (ndc_id = 0)
		result := db.Model(&user.User{}).
			Where("uid = ? AND ndc_id = 0", uid).
			Update("push_enabled", req.Enabled)

		if result.Error != nil {
			log.Printf("Error updating push settings: %v", result.Error)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"api:statuscode": 500,
				"api:message":    "Failed to update push settings",
			})
		}

		// If push notifications are disabled, delete all device tokens
		if !req.Enabled {
			deleteResult := db.Where("user_id = ?", uid).Delete(&models.DeviceToken{})
			if deleteResult.Error != nil {
				log.Printf("Error deleting device tokens: %v", deleteResult.Error)
			} else {
				log.Printf("Deleted %d device tokens for user %s (push disabled)", deleteResult.RowsAffected, uid)
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"api:statuscode": 200,
			"api:message":    "Push settings updated successfully",
			"enabled":        req.Enabled,
		})
	}
}
