package util

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

// QRTokenClaims represents claims for QR code authentication tokens
type QRTokenClaims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	ShortCode string `json:"short_code"`
	TokenType string `json:"token_type"` // "qr" to identify QR tokens
	jwt.RegisteredClaims
}

func GenerateToken(userID uuid.UUID, email, provider string) (string, error) {
	// Get JWT expiry from environment (default: 24 hours)
	expiryHours := 24
	if envExpiry := os.Getenv("JWT_EXPIRY_HOURS"); envExpiry != "" {
		if parsed, err := strconv.Atoi(envExpiry); err == nil {
			expiryHours = parsed
		}
	}

	// Get JWT secret from environment
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return "", fmt.Errorf("JWT_SECRET not set in environment")
	}

	// Create claims
	userIDStr := userID.String()
	claims := JWTClaims{
		UserID:   userIDStr,
		Email:    email,
		Provider: provider,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "url-shortener-backend",
			Subject:   userIDStr,
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func ValidateToken(tokenString string) (*JWTClaims, error) {
	// Get JWT secret from environment
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return nil, fmt.Errorf("JWT_SECRET not set in environment")
	}

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Validate token
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract claims")
	}

	return claims, nil
}

func RefreshToken(tokenString string) (string, error) {
	// Validate existing token
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("invalid token for refresh: %w", err)
	}

	// Parse userID back to UUID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID in token: %w", err)
	}

	// Generate new token with same user info
	return GenerateToken(userID, claims.Email, claims.Provider)
}

// GenerateQRToken generates a short-lived JWT token for QR code image requests
// Token includes userID, email, shortCode, and token_type for scoping
// Default expiry is 5 minutes, configurable via QR_TOKEN_EXPIRY_MINUTES environment variable
func GenerateQRToken(userID uuid.UUID, email, shortCode string) (string, error) {
	// Get QR token expiry from environment (default: 5 minutes)
	expiryMinutes := 5
	if envExpiry := os.Getenv("QR_TOKEN_EXPIRY_MINUTES"); envExpiry != "" {
		if parsed, err := strconv.Atoi(envExpiry); err == nil && parsed > 0 {
			expiryMinutes = parsed
		}
	}

	// Get JWT secret from environment
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return "", fmt.Errorf("JWT_SECRET not set in environment")
	}

	// Create claims
	userIDStr := userID.String()
	claims := QRTokenClaims{
		UserID:    userIDStr,
		Email:     email,
		ShortCode: shortCode,
		TokenType: "qr",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "url-shortener-backend",
			Subject:   userIDStr,
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign QR token: %w", err)
	}

	return tokenString, nil
}

// ValidateQRToken validates a QR token and returns its claims
// Returns error if token is invalid, expired, or not a QR token
func ValidateQRToken(tokenString string) (*QRTokenClaims, error) {
	// Get JWT secret from environment
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return nil, fmt.Errorf("JWT_SECRET not set in environment")
	}

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &QRTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse QR token: %w", err)
	}

	// Validate token
	if !token.Valid {
		return nil, fmt.Errorf("invalid QR token")
	}

	// Extract claims
	claims, ok := token.Claims.(*QRTokenClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract QR token claims")
	}

	// Validate token type
	if claims.TokenType != "qr" {
		return nil, fmt.Errorf("invalid token type: expected 'qr', got '%s'", claims.TokenType)
	}

	return claims, nil
}
