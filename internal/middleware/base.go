package middleware

import (
	"strings"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/gofiber/fiber/v3"
)

func ValidatePostFields(c fiber.Ctx) error {
	path := c.Path()

	// Skip validation for certain endpoints
	skipValidation := strings.Contains(path, "media/upload") ||
		strings.Contains(path, "live-room") ||
		strings.Contains(path, "web-socket") ||
		strings.Contains(path, "/auth/") ||
		strings.Contains(path, "/notification/")

	if c.Method() == "POST" && !skipValidation {
		var body map[string]interface{}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
		}

		if _, hasTimestamp := body["timestamp"]; !hasTimestamp {
			return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
		}
	}

	return c.Next()
}
