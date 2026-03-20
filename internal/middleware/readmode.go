package middleware

import (
	"strconv"
	"strings"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	"github.com/gofiber/fiber/v3"
)

// ReadModeMiddleware blocks write requests (POST/PUT/PATCH/DELETE) for users in read mode.
// GET/OPTIONS/HEAD always pass through.
// Certain paths (auth, notifications, device-token) are exempt.
func ReadModeMiddleware(c fiber.Ctx) error {
	method := c.Method()
	if method == "GET" || method == "OPTIONS" || method == "HEAD" {
		return c.Next()
	}

	// Skip paths that must always work even in read mode
	path := c.Path()
	if strings.Contains(path, "/auth/") ||
		strings.Contains(path, "/notification/") ||
		strings.Contains(path, "/device-token") {
		return c.Next()
	}

	// Get user UID (may be empty before auth)
	uid := GetAUIDFromContext(c)
	if uid == "" {
		return c.Next()
	}

	db := GetDBFromContext(c)
	if db == nil {
		return c.Next()
	}

	readModeSvc := service.NewReadModeService(db, nil)

	// Determine ndcID from path params
	ndcID := 0
	comIdStr := c.Params("comId")
	if comIdStr != "" {
		if id, err := strconv.Atoi(comIdStr); err == nil {
			ndcID = id
		}
	}

	inReadMode, _ := readModeSvc.IsInReadMode(uid, ndcID)
	if !inReadMode {
		return c.Next()
	}

	return c.Status(fiber.StatusForbidden).JSON(response.NewError(response.StatusReadMode))
}
