package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

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

	// Initialize rate limiters
	middleware.InitRateLimiters()

	// Setup error logging to file if configured
	if cfg.Logging.ErrorLogFile != "" {
		logFile, err := os.OpenFile(cfg.Logging.ErrorLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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
		&user.ReadMode{},
		&chat.Thread{},
		&chat.Message{},
		&chat.ThreadMember{},
		&chat.LiveRoom{},
		&chat.Sticker{},
		&chat.StickerCollection{},
		&chat.UserStickerCollection{},
		&chat.ChatBubble{},
		&chat.UserChatBubble{},
		&chat.UserFrameCollection{},
		&community.AdminLog{},
		&community.Community{},
		&community.FeaturedCommunity{},
		&community.Banner{},
		&blog.Blog{},
		&blog.FeaturedPost{},
		&blog.Vote{},
		&blog.CommentVote{},
		&apikey.APIKey{},
		&apikey.APIKeyUsage{},
		&notification.Notification{},
		&models.DeviceToken{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Data migration: convert role=1000 to is_astranet flag
	migrateAstranetRole(db)

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

	// Start background worker for read mode auto-expiry
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			readModeSvc := service.NewReadModeService(db, nil)
			readModeSvc.CleanupExpiredReadModes()
		}
	}()

	app := fiber.New(fiber.Config{
		BodyLimit:   50 * 1024 * 1024, // 50 MB max body size
		ProxyHeader: fiber.HeaderXForwardedFor,
		TrustProxy:  true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: []string{"172.16.0.0/12", "10.0.0.0/8", "192.168.0.0/16"},
		},
	})

	// Open Graph routes - before middleware to return raw HTML
	opengraph.SetDB(db)
	app.Get("/og/c/:comId", opengraph.HandleCommunity)
	app.Get("/og/c/:comId/t/:threadId", opengraph.HandleChat)
	app.Get("/og/c/:comId/p/:postId", opengraph.HandlePost)
	app.Get("/og/t/:threadId", opengraph.HandleChat)
	app.Get("/og/u/:userId", opengraph.HandleUser)

	// Handle OPTIONS preflight explicitly before any other middleware.
	// Fiber's CORS middleware may pass OPTIONS through if Origin header
	// is missing, causing downstream middleware (APIKey etc.) to reject it.
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == "OPTIONS" {
			c.Set("Access-Control-Allow-Origin", "*")
			c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Accept-Language, Authorization, X-API-Key, NDCAUTH, AUID, NDCDEVICEID, NDC-MSG-SIG, DPoP, X-Timestamp, X-Nonce")
			c.Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
			c.Set("Access-Control-Max-Age", "86400")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	})

	// CORS headers for non-preflight requests
	allowedOrigins := []string{"*"}
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
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
		MaxAge:           86400,
	}))
	app.Use(middleware.SecurityHeadersMiddleware)
	app.Use(middleware.RateLimitMiddleware)
	app.Use(middleware.ConfigMiddleware(&cfg))
	app.Use(middleware.DatabaseMiddleware(db))
	app.Use(middleware.ValidatePostFields)
	app.Use(middleware.ResponseWrapper)
	app.Use(middleware.ReadModeMiddleware)

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

// migrateAstranetRole converts users with role=1000 to is_astranet=true flag.
// This is a one-time migration that runs on startup. It's idempotent.
func migrateAstranetRole(db *gorm.DB) {
	// Find all global profiles with role=1000
	var count int64
	db.Model(&user.User{}).Where("role = ? AND ndc_id = 0", 1000).Count(&count)
	if count == 0 {
		return
	}

	log.Printf("[Migration] Found %d users with role=1000, migrating to is_astranet flag...", count)

	// Get UIDs of all Astranet users
	var uids []string
	db.Model(&user.User{}).Where("role = ? AND ndc_id = 0", 1000).Pluck("uid", &uids)

	db.Transaction(func(tx *gorm.DB) error {
		// Set is_astranet=true on ALL profiles (global + community) for these users
		for _, uid := range uids {
			tx.Model(&user.User{}).Where("uid = ?", uid).Update("is_astranet", true)
		}

		// Reset global profiles from role=1000 to role=0 (Member)
		tx.Model(&user.User{}).Where("role = ? AND ndc_id = 0", 1000).Update("role", 0)

		// For community profiles with role=1000: set to Agent (110) if they are the community agent,
		// otherwise set to Member (0)
		for _, uid := range uids {
			// Get communities where this user is the agent
			var agentNdcIds []int
			tx.Model(&community.Community{}).
				Where("json_extract(agent, '$.uid') = ?", uid).
				Pluck("ndc_id", &agentNdcIds)

			if len(agentNdcIds) > 0 {
				// Set Agent role for communities where user is the agent
				tx.Model(&user.User{}).
					Where("uid = ? AND ndc_id IN ? AND role = ?", uid, agentNdcIds, 1000).
					Update("role", user.RoleAgent)
			}

			// Set remaining role=1000 profiles to Member
			tx.Model(&user.User{}).
				Where("uid = ? AND ndc_id != 0 AND role = ?", uid, 1000).
				Update("role", user.RoleMember)
		}

		return nil
	})

	log.Printf("[Migration] Successfully migrated %d Astranet team members", count)
}
