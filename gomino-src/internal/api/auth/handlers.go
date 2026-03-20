package auth

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/internal/service"
	jwtUtil "github.com/AugustLigh/GoMino/pkg/jwt"
	"github.com/AugustLigh/GoMino/pkg/smtp"
	"github.com/gofiber/fiber/v3"
)

var (
	captchaService *service.CaptchaService
	smtpService    *smtp.Service
)

// InitValidationServices инициализирует сервисы для валидации
func InitValidationServices(smtpConfig smtp.Config) {
	captchaService = service.NewCaptchaService()
	smtpService = smtp.NewService(smtpConfig)
}

// Login godoc
// @Summary Login to the application
// @Description Authenticate user with email and secret
// @Tags auth
// @Accept  json
// @Produce  json
// @Param   request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/auth/login [post]
func Login(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	cfg := middleware.GetConfigFromContext(c)

	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	// Извлекаем пароль из secret (формат: "0 $password")
	password := strings.TrimPrefix(req.Secret, "0 ")
	if password == req.Secret {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	userService := service.NewUserService(db)
	user, err := userService.AuthenticateUser(req.Email, password)
	if err != nil {
		log.Printf("Authentication failed for email %s: %v", req.Email, err)
		if errors.Is(err, service.ErrUserNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(response.NewError(response.StatusAccountNotExist))
		}
		return c.Status(fiber.StatusUnauthorized).JSON(response.InvalidCredentials())
	}

	jwtSecret := cfg.JWT.Secret
	if jwtSecret == "" {
		log.Printf("JWT_SECRET not configured")
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// DPoP is required for all logins, unless bypassed by API key
	if !middleware.ShouldBypassSecurity(c) && (req.DPoPPublicKey == "" || req.DPoPKeyID == "") {
		log.Printf("[Auth] DPoP keys not provided for login from email %s", req.Email)
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusDPoPLoginRequired))
	}

	var dpopInfo *jwtUtil.DPoPInfo
	if req.DPoPPublicKey != "" && req.DPoPKeyID != "" {
		dpopInfo = &jwtUtil.DPoPInfo{
			PublicKey: req.DPoPPublicKey,
			KeyID:     req.DPoPKeyID,
		}
	}

	sid, err := jwtUtil.GenerateTokenWithDPoP(user.UID, req.Email, jwtSecret, dpopInfo)
	if err != nil {
		log.Printf("Error generating token for user %s: %v", user.UID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	secret := generateAminoSecret(user.UID, c.IP(), req.DeviceID)

	// Get user auth to retrieve content region
	userAuth, _ := userService.GetUserAuthByEmail(req.Email)
	contentRegion := "en"
	if userAuth != nil && userAuth.ContentRegion != "" {
		contentRegion = userAuth.ContentRegion
	}

	return c.JSON(NewLoginResponse(user, req.Email, sid, secret, contentRegion))
}

// Register godoc
// @Summary Register a new user
// @Description Register a new user with email, nickname, and secret
// @Tags auth
// @Accept  json
// @Produce  json
// @Param   request body RegisterRequest true "Registration data"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/auth/register [post]
func Register(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	cfg := middleware.GetConfigFromContext(c)

	var req RegisterRequest
	if err := c.Bind().Body(&req); err != nil {
		log.Printf("Failed to parse register request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	if err := req.Validate(); err != nil {
		log.Printf("Register validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	// Проверяем капчу
	verificationCode := req.ValidationContext.Data.Code
	if !captchaService.VerifyCaptcha(req.Email, verificationCode) {
		log.Printf("Invalid verification code for email: %s", req.Email)
		return c.Status(fiber.StatusOK).JSON(response.NewError(response.StatusInvalidVerifyCode))
	}

	password := strings.TrimPrefix(req.Secret, "0 ")
	if password == req.Secret {
		log.Printf("Invalid secret format")
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	userService := service.NewUserService(db)
	user, err := userService.CreateUserWithAuth(req.Email, password, req.Nickname)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		if err == service.ErrEmailAlreadyExists {
			return c.Status(fiber.StatusOK).JSON(response.NewError(response.StatusEmailAlreadyTaken))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	jwtSecret := cfg.JWT.Secret
	if jwtSecret == "" {
		log.Printf("JWT_SECRET not configured")
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// DPoP is required for all registrations (security measure), unless bypassed by API key
	if !middleware.ShouldBypassSecurity(c) && (req.DPoPPublicKey == "" || req.DPoPKeyID == "") {
		log.Printf("DPoP keys not provided for registration from email %s", req.Email)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"api:statuscode": 104,
			"api:message":    "DPoP keys are required for registration",
		})
	}

	var dpopInfo *jwtUtil.DPoPInfo
	if req.DPoPPublicKey != "" && req.DPoPKeyID != "" {
		dpopInfo = &jwtUtil.DPoPInfo{
			PublicKey: req.DPoPPublicKey,
			KeyID:     req.DPoPKeyID,
		}
	}

	sid, err := jwtUtil.GenerateTokenWithDPoP(user.UID, req.Email, jwtSecret, dpopInfo)
	if err != nil {
		log.Printf("Error generating token for user %s: %v", user.UID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	secret := generateAminoSecret(user.UID, c.IP(), req.DeviceID)

	// New users start with default "en" content region
	return c.Status(fiber.StatusOK).JSON(NewLoginResponse(user, req.Email, sid, secret, "en"))
}

// Validation godoc
// @Summary Request security validation
// @Description Send a validation code to the email
// @Tags auth
// @Accept  json
// @Produce  json
// @Param   request body SecurityValidationRequest true "Validation request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/auth/request-security-validation [post]
func Validation(c fiber.Ctx) error {
	var req SecurityValidationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	captcha, err := captchaService.GenerateCaptcha(req.Identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	if err := smtpService.SendCaptchaEmail(req.Identity, captcha.Image); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{})
}

// RefreshToken godoc
// @Summary Refresh access token using secret
// @Description Exchange amino secret for a new JWT sid and secret
// @Tags auth
// @Accept  json
// @Produce  json
// @Param   request body RefreshTokenRequest true "Refresh token request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /g/s/auth/refresh [post]
func RefreshToken(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	cfg := middleware.GetConfigFromContext(c)

	var req RefreshTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	// Parse the amino secret to extract UID
	// Format: "31 <random10> <uid> <ip> <deviceHash> 1 <timestamp> <signature>"
	parts := strings.Fields(req.Secret)
	if len(parts) < 8 || parts[0] != "31" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.InvalidCredentials())
	}

	uid := parts[2]
	secretTimestamp := int64(0)
	fmt.Sscanf(parts[6], "%d", &secretTimestamp)

	// Validate secret is not too old (30 days max)
	if time.Now().Unix()-secretTimestamp > 30*24*3600 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"api:statuscode": 103,
			"api:message":    "Refresh token expired",
		})
	}

	// Verify the signature component
	signData := fmt.Sprintf("%s%s%d", uid, parts[3], secretTimestamp)
	h := sha1.New()
	h.Write([]byte(signData))
	expectedSig := hex.EncodeToString(h.Sum(nil))[:20]
	if parts[7] != expectedSig {
		return c.Status(fiber.StatusUnauthorized).JSON(response.InvalidCredentials())
	}

	// Look up user (global profile, ndcId=0)
	userService := service.NewUserService(db)
	user, err := userService.GetUserByID(uid, 0, true)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.InvalidCredentials())
	}

	// Get user auth to retrieve email and content region
	userAuth, err := userService.GetUserAuthByUID(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.InvalidCredentials())
	}

	jwtSecret := cfg.JWT.Secret
	if jwtSecret == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// DPoP is required for token refresh, unless bypassed by API key
	if !middleware.ShouldBypassSecurity(c) && (req.DPoPPublicKey == "" || req.DPoPKeyID == "") {
		log.Printf("[Auth] DPoP keys not provided for token refresh for user %s", uid)
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusDPoPRefreshRequired))
	}

	var dpopInfo *jwtUtil.DPoPInfo
	if req.DPoPPublicKey != "" && req.DPoPKeyID != "" {
		dpopInfo = &jwtUtil.DPoPInfo{
			PublicKey: req.DPoPPublicKey,
			KeyID:     req.DPoPKeyID,
		}
	}

	sid, err := jwtUtil.GenerateTokenWithDPoP(user.UID, userAuth.Email, jwtSecret, dpopInfo)
	if err != nil {
		log.Printf("Error generating refresh token for user %s: %v", user.UID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	newSecret := generateAminoSecret(user.UID, c.IP(), req.DeviceID)

	contentRegion := "en"
	if userAuth.ContentRegion != "" {
		contentRegion = userAuth.ContentRegion
	}

	return c.JSON(NewLoginResponse(user, userAuth.Email, sid, newSecret, contentRegion))
}

func ResetPassword(c fiber.Ctx) error {
	return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
} // TODO реализовать сброс пароля

func generateAminoSecret(uid, ip, deviceID string) string {
	timestamp := time.Now().Unix()

	h := sha1.New()
	h.Write([]byte(deviceID))
	deviceHash := hex.EncodeToString(h.Sum(nil))

	signData := fmt.Sprintf("%s%s%d", uid, ip, timestamp)
	h = sha1.New()
	h.Write([]byte(signData))
	signature := hex.EncodeToString(h.Sum(nil))[:20]

	return fmt.Sprintf("31 %s %s %s %s 1 %d %s",
		generateRandomString(10), uid, ip, deviceHash, timestamp, signature)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// UpdateContentRegion godoc
// @Summary Update user content region preference
// @Description Update the content region (language segment) for the authenticated user
// @Tags account
// @Accept  json
// @Produce  json
// @Param   request body object{contentRegion=string} true "Content region (ru, en, es, ar)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/account/content-region [post]
func UpdateContentRegion(c fiber.Ctx) error {
	db := middleware.GetDBFromContext(c)
	auid := middleware.GetAUIDFromContext(c)

	var req struct {
		ContentRegion string `json:"contentRegion"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
	}

	// Validate content region
	validRegions := map[string]bool{"ru": true, "en": true, "es": true, "ar": true}
	if !validRegions[req.ContentRegion] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"api:statuscode": 104,
			"api:message":    "Invalid content region. Must be one of: ru, en, es, ar",
		})
	}

	userService := service.NewUserService(db)
	if err := userService.UpdateContentRegion(auid, req.ContentRegion); err != nil {
		log.Printf("Failed to update content region for user %s: %v", auid, err)
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.JSON(fiber.Map{
		"api:statuscode": 0,
		"api:message":    "Content region updated successfully",
		"contentRegion":  req.ContentRegion,
	})
}
