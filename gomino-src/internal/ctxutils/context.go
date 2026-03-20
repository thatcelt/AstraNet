package ctxutils

import (
	"strconv"

	"github.com/AugustLigh/GoMino/pkg/config"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// GetConfigFromContext retrieves the config from fiber context
func GetConfigFromContext(c fiber.Ctx) *config.Config {
	cfg, ok := c.Locals("config").(*config.Config)
	if !ok {
		return nil
	}
	return cfg
}

// GetDBFromContext retrieves the database from fiber context
func GetDBFromContext(c fiber.Ctx) *gorm.DB {
	db, ok := c.Locals("db").(*gorm.DB)
	if !ok {
		return nil
	}
	return db
}

// GetComIdFromContext retrieves the community ID from route params
// Returns 0 for global context (routes starting with /g/s/)
func GetComIdFromContext(c fiber.Ctx) int {
	comIdStr := c.Params("comId")
	if comIdStr == "" {
		return 0 // Global context
	}

	comId, err := strconv.Atoi(comIdStr)
	if err != nil {
		return 0
	}

	return comId
}
