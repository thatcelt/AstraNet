// internal/middleware/config.go
package middleware

import (
	"github.com/AugustLigh/GoMino/pkg/config"
	"github.com/gofiber/fiber/v3"
)

func ConfigMiddleware(cfg *config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("config", cfg)
		return c.Next()
	}
}

func GetConfigFromContext(c fiber.Ctx) *config.Config {
	cfg, ok := c.Locals("config").(*config.Config)
	if !ok {
		return nil
	}
	return cfg
}
