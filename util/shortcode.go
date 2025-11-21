package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// GenerateShortCode generates a unique 6-character short code
func GenerateShortCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 6

	// Generate random short code
	shortCode := make([]byte, length)
	for i := range shortCode {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to simple random if crypto/rand fails
			shortCode[i] = charset[randomIndex.Int64()]
		} else {
			shortCode[i] = charset[randomIndex.Int64()]
		}
	}

	return string(shortCode)
}

// GetShortURLBase returns the base URL for short links from environment variable
// Priority: FRONTEND_URL > SHORT_URL_BASE > BASE_URL > default
// Falls back to default production URL if not set
func GetShortURLBase() string {
	// Check FRONTEND_URL first (where short links are actually served to users)
	if baseURL := os.Getenv("FRONTEND_URL"); baseURL != "" {
		return strings.TrimSuffix(baseURL, "/")
	}
	// Default fallback to production URL
	return "https://www.sniply.co.in"
}

// GetShortURLBaseFromRequest automatically detects the base URL from the HTTP request
// Priority: FRONTEND_URL > detected request URL > SHORT_URL_BASE > default
// Falls back to environment variable or default if detection fails
func GetShortURLBaseFromRequest(c *gin.Context) string {
	// First check FRONTEND_URL (where short links are actually served)
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		return strings.TrimSuffix(frontendURL, "/")
	}
	
	if c == nil || c.Request == nil {
		return GetShortURLBase()
	}

	// Try to detect from request (fallback if FRONTEND_URL not set)
	req := c.Request
	
	// Get scheme (http or https)
	scheme := "https"
	if req.TLS == nil {
		// Check X-Forwarded-Proto header (common in reverse proxy setups)
		if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}

	// Get host from request
	host := req.Host
	if host == "" {
		// Fallback to environment variable
		return GetShortURLBase()
	}

	// Construct base URL from request
	baseURL := fmt.Sprintf("%s://%s", scheme, host)
	return strings.TrimSuffix(baseURL, "/")
}

// BuildShortURL constructs the full short URL from a short code
// Uses environment variable or default
func BuildShortURL(code string) string {
	baseURL := GetShortURLBase()
	return fmt.Sprintf("%s/%s", baseURL, code)
}

// BuildShortURLFromRequest constructs the full short URL from a short code
// Automatically detects base URL from the HTTP request
func BuildShortURLFromRequest(c *gin.Context, code string) string {
	baseURL := GetShortURLBaseFromRequest(c)
	return fmt.Sprintf("%s/%s", baseURL, code)
}