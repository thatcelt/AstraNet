package dpop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidDPoPProof  = errors.New("invalid DPoP proof")
	ErrDPoPExpired       = errors.New("DPoP proof expired")
	ErrDPoPMethodMismatch = errors.New("DPoP method mismatch")
	ErrDPoPURLMismatch    = errors.New("DPoP URL mismatch")
	ErrDPoPTokenMismatch  = errors.New("DPoP token hash mismatch")
)

// DPoPHeader represents the JWT header of a DPoP proof
type DPoPHeader struct {
	Typ string         `json:"typ"`
	Alg string         `json:"alg"`
	JWK map[string]any `json:"jwk"`
}

// DPoPPayload represents the JWT payload of a DPoP proof
type DPoPPayload struct {
	JTI string `json:"jti"` // Unique token ID
	HTM string `json:"htm"` // HTTP method
	HTU string `json:"htu"` // HTTP URI
	IAT int64  `json:"iat"` // Issued at
	EXP int64  `json:"exp"` // Expiration
	ATH string `json:"ath"` // Access token hash (optional)
}

// UserDPoPInfo stores user's DPoP key information
type UserDPoPInfo struct {
	PublicKey string `json:"dpopPublicKey"`
	KeyID     string `json:"dpopKeyId"`
}

// ValidateDPoPProof validates a DPoP proof token
func ValidateDPoPProof(proof, method, fullURL string, userDPoP *UserDPoPInfo, accessToken string) error {
	if proof == "" {
		return nil // DPoP is optional, skip if not provided
	}

	if userDPoP == nil || userDPoP.PublicKey == "" {
		return nil // User doesn't have DPoP configured, skip
	}

	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		return ErrInvalidDPoPProof
	}

	// Decode header
	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return fmt.Errorf("%w: invalid header encoding", ErrInvalidDPoPProof)
	}

	var header DPoPHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return fmt.Errorf("%w: invalid header JSON", ErrInvalidDPoPProof)
	}

	// Verify header
	if header.Typ != "dpop+jwt" {
		return fmt.Errorf("%w: wrong type", ErrInvalidDPoPProof)
	}

	if header.Alg != "HS256" {
		return fmt.Errorf("%w: unsupported algorithm", ErrInvalidDPoPProof)
	}

	// Verify JWK matches user's key
	if header.JWK == nil {
		return fmt.Errorf("%w: missing JWK", ErrInvalidDPoPProof)
	}

	jwkKey, ok := header.JWK["k"].(string)
	if !ok || jwkKey != userDPoP.PublicKey {
		return fmt.Errorf("%w: key mismatch", ErrInvalidDPoPProof)
	}

	jwkKid, ok := header.JWK["kid"].(string)
	if !ok || jwkKid != userDPoP.KeyID {
		return fmt.Errorf("%w: key ID mismatch", ErrInvalidDPoPProof)
	}

	// Decode payload
	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return fmt.Errorf("%w: invalid payload encoding", ErrInvalidDPoPProof)
	}

	var payload DPoPPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("%w: invalid payload JSON", ErrInvalidDPoPProof)
	}

	// Verify timing
	now := time.Now().Unix()
	if payload.EXP < now {
		return ErrDPoPExpired
	}

	// Allow 5 minutes clock skew for IAT
	if payload.IAT > now+300 {
		return fmt.Errorf("%w: issued in future", ErrInvalidDPoPProof)
	}

	// Verify method
	if strings.ToUpper(payload.HTM) != strings.ToUpper(method) {
		return ErrDPoPMethodMismatch
	}

	// Verify URL (normalize both)
	proofURL := normalizeURL(payload.HTU)
	requestURL := normalizeURL(fullURL)
	if proofURL != requestURL {
		return ErrDPoPURLMismatch
	}

	// Verify access token hash if present
	if accessToken != "" && payload.ATH != "" {
		expectedATH := sha256Hash(accessToken)
		if payload.ATH != expectedATH {
			return ErrDPoPTokenMismatch
		}
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig, err := base64URLDecode(parts[2])
	if err != nil {
		return fmt.Errorf("%w: invalid signature encoding", ErrInvalidDPoPProof)
	}

	// Decode public key to use as HMAC secret
	publicKeyBytes, err := base64URLDecode(userDPoP.PublicKey)
	if err != nil {
		return fmt.Errorf("%w: invalid public key", ErrInvalidDPoPProof)
	}

	mac := hmac.New(sha256.New, publicKeyBytes)
	mac.Write([]byte(signingInput))
	calculatedSig := mac.Sum(nil)

	if !hmac.Equal(expectedSig, calculatedSig) {
		return fmt.Errorf("%w: signature verification failed", ErrInvalidDPoPProof)
	}

	return nil
}

// sha256Hash computes SHA256 hash and returns base64url encoded result
func sha256Hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// normalizeURL normalizes URL for comparison (removes query and fragment)
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Build normalized URL
	normalized := u.Scheme + "://" + u.Host
	if u.Port() != "" && u.Port() != "80" && u.Port() != "443" {
		normalized += ":" + u.Port()
	}
	normalized += u.Path

	return normalized
}

// base64URLDecode decodes base64url without padding
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	return base64.URLEncoding.DecodeString(s)
}
