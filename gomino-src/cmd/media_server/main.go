package main

import (
	"log"
	"os"
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

	app := fiber.New(fiber.Config{
		AppName: "GoMino Media Server",
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "HEAD", "OPTIONS"},
		MaxAge:       86400,
	}))

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
