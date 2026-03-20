package middleware

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/AugustLigh/GoMino/pkg/dpop"
	"github.com/AugustLigh/GoMino/pkg/jwt"
	"github.com/gofiber/fiber/v3"
)

const (
	// Maximum age for timestamp (5 minutes)
	maxTimestampAge = 5 * 60 * 1000 // 5 minutes in milliseconds
	// Signature prefix byte
	signaturePrefix byte = 25
)

// DPoPMiddleware validates DPoP proofs for requests
func DPoPMiddleware(c fiber.Ctx) error {
	log.Printf("[DPoPMiddleware] Path: %s, Method: %s", c.Path(), c.Method())
	// Check if API key is present and security should be bypassed
	if ShouldBypassSecurity(c) {
		log.Printf("[DPoPMiddleware] Bypassing security")
		return c.Next()
	}

	// Get the access token from NDCAUTH header
	sid := c.Get("NDCAUTH")
	if strings.HasPrefix(sid, "sid=") {
		sid = strings.TrimPrefix(sid, "sid=")
	}

	if sid == "" {
		// No token, skip DPoP validation (auth middleware will handle this)
		return c.Next()
	}

	// Validate the token to get DPoP info
	cfg := GetConfigFromContext(c)
	if cfg == nil {
		return c.Next()
	}

	claims, err := jwt.ValidateToken(sid, cfg.JWT.Secret)
	if err != nil {
		// Token invalid, auth middleware will handle this
		return c.Next()
	}

	// Token MUST have DPoP binding — reject old tokens without it
	if claims.DPoPKeyID == "" {
		log.Printf("[DPoPMiddleware] Token has no DPoP binding for user %s", claims.UserID)
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusDPoPBindingMissing))
	}

	dpopProof := c.Get("DPoP")

	// DPoP proof header is required
	if dpopProof == "" {
		log.Printf("[DPoPMiddleware] DPoP proof required but missing for user with DPoP binding")
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusDPoPBindingMissing))
	}

	userDPoP := &dpop.UserDPoPInfo{
		PublicKey: claims.DPoPThumbprint,
		KeyID:     claims.DPoPKeyID,
	}

	// Build full URL - use X-Forwarded-Proto if behind reverse proxy
	scheme := c.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = string(c.Request().URI().Scheme())
	}
	if scheme == "" {
		scheme = "https" // Default to https for production
	}
	fullURL := scheme + "://" + string(c.Request().Host()) + string(c.Request().URI().Path())

	log.Printf("[DPoPMiddleware] Validating DPoP proof for URL: %s", fullURL)

	err = dpop.ValidateDPoPProof(dpopProof, c.Method(), fullURL, userDPoP, sid)
	if err != nil {
		log.Printf("[DPoPMiddleware] DPoP validation failed: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	return c.Next()
}

// SignatureMiddleware validates NDC-MSG-SIG header for POST/PUT/PATCH requests
func SignatureMiddleware(c fiber.Ctx) error {
	// Check if API key is present and security should be bypassed
	if ShouldBypassSecurity(c) {
		return c.Next()
	}

	// Only validate for methods with body
	method := c.Method()
	if method != "POST" && method != "PUT" && method != "PATCH" {
		return c.Next()
	}

	signature := c.Get("NDC-MSG-SIG")
	timestampStr := c.Get("X-Timestamp")
	nonce := c.Get("X-Nonce")

	// If no signature headers, skip validation (backward compatibility)
	if signature == "" {
		return c.Next()
	}

	// Validate timestamp to prevent replay attacks
	if timestampStr != "" {
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.InvalidRequest())
		}

		now := time.Now().UnixMilli()
		if now-timestamp > maxTimestampAge {
			// Request is too old
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"api:statuscode": 104,
				"api:message":    "Request timestamp expired",
			})
		}

		if timestamp > now+60000 { // Allow 1 minute future clock skew
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"api:statuscode": 104,
				"api:message":    "Request timestamp in future",
			})
		}
	}

	// Get request body for signature verification
	body := c.Body()
	if len(body) == 0 {
		return c.Next()
	}

	// For now, we log the signature but don't enforce validation
	// because the key derivation requires server-side stored user salt
	// This is a placeholder for full implementation
	c.Locals("signature", signature)
	c.Locals("nonce", nonce)

	return c.Next()
}

// VerifySignature verifies the NDC-MSG-SIG against the body
// This requires the user's device salt which should be stored server-side
func VerifySignature(signature string, body []byte, timestamp int64, userSalt string) bool {
	if signature == "" || len(body) == 0 {
		return false
	}

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	// Check prefix
	if len(sigBytes) < 21 || sigBytes[0] != signaturePrefix {
		return false
	}

	// Extract the actual HMAC from signature
	actualHMAC := sigBytes[1:]

	// Derive signature key (must match client's derivation)
	sigKey := deriveSigKey(userSalt, timestamp)

	// Create data with timestamp for verification
	dataWithTime := fmt.Sprintf("%s|%d", string(body), timestamp)

	// Calculate expected HMAC
	mac := hmac.New(sha1.New, sigKey)
	mac.Write([]byte(dataWithTime))
	expectedHMAC := mac.Sum(nil)

	return hmac.Equal(actualHMAC, expectedHMAC)
}

// deriveSigKey derives the signature key from user salt and time
func deriveSigKey(userSalt string, timestamp int64) []byte {
	components := make([]byte, 0, 20)

	// Base component from device salt
	if userSalt != "" {
		saltHash := sha256.Sum256([]byte(userSalt))
		components = append(components, saltHash[:10]...)
	}

	// Time-based component (hourly rotation, matching client)
	hourlyKey := timestamp / 3600000
	timeData := fmt.Sprintf("sig_%d", hourlyKey)
	timeHash := sha256.Sum256([]byte(timeData))
	components = append(components, timeHash[:10]...)

	return components
}

// RequestBodyParser is a helper to parse and validate request body
type RequestBodyParser struct{}

// ParseAndValidate parses JSON body and stores original for signature verification
func (p *RequestBodyParser) ParseAndValidate(c fiber.Ctx, dest interface{}) error {
	body := c.Body()

	if err := json.Unmarshal(body, dest); err != nil {
		return err
	}

	// Store original body for signature verification
	c.Locals("originalBody", body)

	return nil
}
