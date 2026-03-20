package middleware

import (
	"strconv"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// BanCheckMiddleware blocks requests from banned users.
// Must be placed AFTER AuthMiddleware so that uid is available in context.
// Checks global ban and community ban (when comId is present in route params).
func BanCheckMiddleware(c fiber.Ctx) error {
	uid := GetAUIDFromContext(c)
	if uid == "" {
		return c.Next()
	}

	db := GetDBFromContext(c)
	if db == nil {
		return c.Next()
	}

	// Determine community ID from route params
	ndcID := 0
	comIdStr := c.Params("comId")
	if comIdStr != "" {
		if id, err := strconv.Atoi(comIdStr); err == nil {
			ndcID = id
		}
	}

	permSvc := service.NewPermissionService(db)
	if permSvc.IsBanned(uid, ndcID) {
		return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusUserBanned))
	}

	return c.Next()
}
