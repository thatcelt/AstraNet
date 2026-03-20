package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims представляет JWT claims
type Claims struct {
	UserID       string `json:"userId"`
	Email        string `json:"email"`
	DPoPKeyID    string `json:"dpopKid,omitempty"`    // DPoP Key ID for token binding
	DPoPThumbprint string `json:"dpopJkt,omitempty"` // DPoP JWK Thumbprint
	jwt.RegisteredClaims
}

// DPoPInfo contains DPoP key information for token binding
type DPoPInfo struct {
	PublicKey string
	KeyID     string
}

// GenerateToken генерирует JWT токен на 24 часа
func GenerateToken(userID, email, secret string) (string, error) {
	return GenerateTokenWithDPoP(userID, email, secret, nil)
}

// GenerateTokenWithDPoP генерирует JWT токен с привязкой к DPoP ключу
func GenerateTokenWithDPoP(userID, email, secret string, dpop *DPoPInfo) (string, error) {
	// Устанавливаем claims
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Add DPoP binding if provided
	if dpop != nil && dpop.KeyID != "" {
		claims.DPoPKeyID = dpop.KeyID
		claims.DPoPThumbprint = dpop.PublicKey // Store thumbprint for verification
	}

	// Создаём токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен секретным ключом
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken проверяет и парсит JWT токен
func ValidateToken(tokenString, secret string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
