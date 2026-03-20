package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	mediaStoragePath := getEnv("MEDIA_STORAGE_PATH", "/data/media")
	port := getEnv("MEDIA_SERVER_PORT", "8081")
	allowedOrigin := getEnv("MEDIA_ALLOWED_ORIGIN", "*")

	// Validate storage path exists and is a directory
	absPath, err := filepath.Abs(mediaStoragePath)
	if err != nil {
		log.Fatalf("Invalid MEDIA_STORAGE_PATH: %v", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		log.Fatalf("MEDIA_STORAGE_PATH does not exist: %s", absPath)
	}
	if !info.IsDir() {
		log.Fatalf("MEDIA_STORAGE_PATH is not a directory: %s", absPath)
	}

	// Resolve symlinks to ensure we know the real path
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		log.Fatalf("Failed to resolve MEDIA_STORAGE_PATH symlinks: %v", err)
	}
	mediaStoragePath = realPath

	app := fiber.New(fiber.Config{
		AppName: "GoMino Media Server",
	})

	// CORS: configurable allowed origin
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{allowedOrigin},
		AllowMethods: []string{"GET", "HEAD", "OPTIONS"},
		MaxAge:       86400,
	}))

	// Security headers
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Content-Security-Policy", "default-src 'none'")
		c.Set("Referrer-Policy", "no-referrer")
		return c.Next()
	})

	app.Get("/*", static.New(mediaStoragePath, static.Config{
		Compress:      false,
		ByteRange:     true,
		Browse:        false,
		CacheDuration: 365 * 24 * time.Hour,
		MaxAge:        31536000,
	}))

	log.Printf("Media server starting on :%s (Path: %s, Cores: %d)", port, mediaStoragePath, runtime.NumCPU())

	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
