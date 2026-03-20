// internal/middleware/database.go
package middleware

import (
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func DatabaseMiddleware(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("db", db)
		return c.Next()
	}
}

func GetDBFromContext(c fiber.Ctx) *gorm.DB {
	db, ok := c.Locals("db").(*gorm.DB)
	if !ok {
		return nil
	}
	return db
}
