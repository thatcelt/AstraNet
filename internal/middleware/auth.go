package middleware

import (
	"log"
	"strings"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/pkg/jwt"
	"github.com/gofiber/fiber/v3"
)

func AuthMiddleware(c fiber.Ctx) error {
	log.Printf("[AuthMiddleware] Path: %s", c.Path())

	// Получаем SID из заголовка NDCAUTH или Authorization
	sid := c.Get("NDCAUTH")
	if strings.HasPrefix(sid, "sid=") {
		sid = strings.TrimPrefix(sid, "sid=")
	} else if sid == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			sid = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// Получаем AUID из заголовков
	auidHeader := c.Get("AUID")

	log.Printf("[AuthMiddleware] sid empty: %v, auid empty: %v", sid == "", auidHeader == "")

	if sid == "" || auidHeader == "" {
		log.Printf("[AuthMiddleware] Missing credentials for path: %s", c.Path())
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	cfg := GetConfigFromContext(c)
	if cfg == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	claims, err := jwt.ValidateToken(sid, cfg.JWT.Secret)
	if err != nil {
		log.Printf("[AuthMiddleware] Token validation failed: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	// Проверяем, что AUID из заголовка совпадает с ID из токена
	if claims.UserID != auidHeader {
		log.Printf("[AuthMiddleware] AUID mismatch: claims.UserID=%s, auidHeader=%s", claims.UserID, auidHeader)
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	log.Printf("[AuthMiddleware] Auth successful for user: %s", claims.UserID)

	// Сохраняем UID в Locals для использования в хендлерах
	c.Locals("uid", claims.UserID)
	c.Locals("auid", claims.UserID) // Keep for backward compatibility

	return c.Next()
}

// OptionalAuthMiddleware парсит токен если он есть, но не блокирует запрос без авторизации.
// Используется для эндпоинтов, где авторизация опциональна (например, чтобы вернуть votedValue).
func OptionalAuthMiddleware(c fiber.Ctx) error {
	sid := c.Get("NDCAUTH")
	if strings.HasPrefix(sid, "sid=") {
		sid = strings.TrimPrefix(sid, "sid=")
	} else if sid == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			sid = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	auidHeader := c.Get("AUID")

	if sid == "" || auidHeader == "" {
		return c.Next()
	}

	cfg := GetConfigFromContext(c)
	if cfg == nil {
		return c.Next()
	}

	claims, err := jwt.ValidateToken(sid, cfg.JWT.Secret)
	if err != nil {
		return c.Next()
	}

	if claims.UserID != auidHeader {
		return c.Next()
	}

	c.Locals("uid", claims.UserID)
	c.Locals("auid", claims.UserID)

	return c.Next()
}

func GetAUIDFromContext(c fiber.Ctx) string {
	auid, ok := c.Locals("auid").(string)
	if !ok {
		return ""
	}
	return auid
}
