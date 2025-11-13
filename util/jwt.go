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
