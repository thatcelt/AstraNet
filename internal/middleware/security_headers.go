package middleware

import (
	"github.com/gofiber/fiber/v3"
)

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware(c fiber.Ctx) error {
	// Prevent MIME type sniffing
	c.Set("X-Content-Type-Options", "nosniff")

	// Prevent clickjacking
	c.Set("X-Frame-Options", "DENY")

	// XSS protection (for older browsers)
	c.Set("X-XSS-Protection", "1; mode=block")

	// Referrer policy
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Permissions policy - disable unnecessary browser features
	c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

	// Cache control for API responses
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Set("Pragma", "no-cache")

	return c.Next()
}
