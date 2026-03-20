package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"gorm.io/gorm"

	"github.com/AugustLigh/GoMino/internal/api/auth"
	"github.com/AugustLigh/GoMino/internal/api/opengraph"
	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/router"
	"github.com/AugustLigh/GoMino/internal/service"
	pushService "github.com/AugustLigh/GoMino/internal/service/push"

	"github.com/AugustLigh/GoMino/internal/models"
	"github.com/AugustLigh/GoMino/internal/models/apikey"
	"github.com/AugustLigh/GoMino/internal/models/blog"
	"github.com/AugustLigh/GoMino/internal/models/chat"
	"github.com/AugustLigh/GoMino/internal/models/community"
	"github.com/AugustLigh/GoMino/internal/models/notification"
	"github.com/AugustLigh/GoMino/internal/models/user"

	"github.com/AugustLigh/GoMino/pkg/config"
	"github.com/AugustLigh/GoMino/pkg/database"
	"github.com/AugustLigh/GoMino/pkg/smtp"
)

// @title GoMino API
// @version 1.0
// @description This is the API server for GoMino.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@gomino.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// NOTE: The @host value above should match API_HOST in your .env file for correct API documentation
func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatal("Cannot load config:", err)
	}

	// Setup error logging to file if configured
	if cfg.Logging.ErrorLogFile != "" {
		logFile, err := os.OpenFile(cfg.Logging.ErrorLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("Warning: Failed to open error log file: %v", err)
		} else {
			// Write to both file and stdout
			multiWriter := io.MultiWriter(os.Stdout, logFile)
			log.SetOutput(multiWriter)
			log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
			log.Printf("Error logging initialized to file: %s", cfg.Logging.ErrorLogFile)
		}
	}

	db, err := database.InitDatabase(cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = db.AutoMigrate(
		&user.User{},
		&user.UserAuth{},
		&user.AvatarFrame{},
		&user.CustomTitle{},
		&user.UserFollow{},
		&user.Comment{},
		&user.GlobalBan{},
		&user.UserBlock{},
		&chat.Thread{},
		&chat.Message{},
		&chat.ThreadMember{},
		&chat.LiveRoom{},
		&community.AdminLog{},
		&community.Community{},
		&community.FeaturedCommunity{},
		&community.Banner{},
		&blog.Blog{},
		&blog.FeaturedPost{},
		&blog.Vote{},
		&apikey.APIKey{},
		&apikey.APIKeyUsage{},
		&notification.Notification{},
		&models.DeviceToken{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Create bootstrap API key if it doesn't exist
	if err := createBootstrapAPIKey(db); err != nil {
		log.Printf("Warning: Failed to create bootstrap API key: %v", err)
	}

	// Инициализируем SMTP сервис для валидации
	smtpConfig := smtp.Config{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	}
	auth.InitValidationServices(smtpConfig)

	// Инициализируем Push Notification Service (Firebase)
	var pushSvc *pushService.PushNotificationService
	if cfg.Firebase.Enabled && cfg.Firebase.ServiceAccountPath != "" {
		var err error
		pushSvc, err = pushService.NewPushNotificationService(db, cfg.Firebase.ServiceAccountPath)
		if err != nil {
			log.Printf("Warning: Failed to initialize Push Notification Service: %v", err)
			log.Println("Push notifications will be disabled")
		} else {
			log.Println("Push Notification Service initialized successfully")
			// Make push service globally available
			service.SetPushService(pushSvc)
		}
	} else {
		log.Println("Push notifications are disabled (check FIREBASE_PUSH_ENABLED and FIREBASE_SERVICE_ACCOUNT_PATH in .env)")
	}

	app := fiber.New()

	// Open Graph routes - before middleware to return raw HTML
	opengraph.SetDB(db)
	app.Get("/og/c/:comId", opengraph.HandleCommunity)
	app.Get("/og/c/:comId/t/:threadId", opengraph.HandleChat)
	app.Get("/og/c/:comId/p/:postId", opengraph.HandlePost)
	app.Get("/og/t/:threadId", opengraph.HandleChat)
	app.Get("/og/u/:userId", opengraph.HandleUser)

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Accept-Language",
			"Authorization",
			"X-API-Key",
			"NDCAUTH",
			"AUID",
			"NDCDEVICEID",
			"NDC-MSG-SIG",
			"DPoP",
			"X-Timestamp",
			"X-Nonce",
		},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: false,
	}))
	app.Use(middleware.ConfigMiddleware(&cfg))
	app.Use(middleware.DatabaseMiddleware(db))
	app.Use(middleware.ValidatePostFields)
	app.Use(middleware.ResponseWrapper)

	// userService := service.NewUserService(db)

	// userService.CreateUser(&models.User{
	// 	UID:      "testuser123",
	// 	Nickname: "ImNumberOne",
	// 	Icon:     "default-icon-url",
	// })

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, user!")
	})

	// Configure LiveKit
	router.SetLiveKitConfig(cfg.LiveKit)

	router.SetupRoutes(app, db, &cfg)

	app.Listen(fmt.Sprintf(":%s", cfg.Server.Port))
}

// createBootstrapAPIKey creates an initial API key for development
func createBootstrapAPIKey(db *gorm.DB) error {
	apiKeyService := service.NewAPIKeyService(db)

	// Check if any API keys exist
	var count int64
	if err := db.Model(&apikey.APIKey{}).Count(&count).Error; err != nil {
		return err
	}

	// If keys already exist, skip bootstrap
	if count > 0 {
		log.Println("API keys already exist, skipping bootstrap")
		return nil
	}

	// Create bootstrap key
	plainKey, key, err := apiKeyService.CreateAPIKey(
		nil,                              // No specific user
		"Bootstrap Developer Key",        // Name
		[]string{"*"},                    // All scopes
		10000,                            // 10k requests per hour
		nil,                              // Never expires
	)
	if err != nil {
		return err
	}

	// Save the key to a file for the developer
	keyFile := "bootstrap_api_key.txt"
	content := fmt.Sprintf(`╔════════════════════════════════════════════════════════════════╗
║           GoMino Bootstrap API Key Generated!                  ║
╚════════════════════════════════════════════════════════════════╝

⚠️  IMPORTANT: Save this key securely - it will not be shown again!

API Key: %s
Key ID:  %s
Name:    %s

This key has been generated for initial development and testing.
You can use this key to:
  1. Make API requests without DPoP/Signature validation
  2. Create additional API keys for specific users
  3. Test your application integrations

Usage:
  - Add header: X-API-Key: %s
  - Or use: Authorization: Bearer %s

Rate Limit: %d requests/hour
Expires: Never

═══════════════════════════════════════════════════════════════

Example Python usage:
  headers = {"X-API-Key": "%s"}
  response = requests.get("http://localhost:8080/api/v1/...", headers=headers)

═══════════════════════════════════════════════════════════════
`,
		plainKey,
		key.ID,
		key.Name,
		plainKey,
		plainKey,
		key.RateLimit,
		plainKey,
	)

	// Write to file
	if err := os.WriteFile(keyFile, []byte(content), 0600); err != nil {
		log.Printf("Warning: Could not write bootstrap key to file: %v", err)
		// Still log to console
		log.Printf("\n%s\n", content)
	} else {
		absPath, _ := filepath.Abs(keyFile)
		log.Printf("\n✅ Bootstrap API key created and saved to: %s\n", absPath)
		log.Printf("📝 Key ID: %s\n", key.ID)
	}

	return nil
}
