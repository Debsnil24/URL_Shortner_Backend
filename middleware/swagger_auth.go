package middleware

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// isAPIRequest checks if the request is an API call (fetch/XHR) that should return JSON instead of redirecting
func isAPIRequest(c *gin.Context) bool {
	// Check if it's an API endpoint
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		return true
	}
	// Check for XHR/fetch request headers
	if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	// Check if Accept header prefers JSON
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	return false
}

// buildEncodedRedirectURL builds and encodes the redirect URL from the request context
// It includes both the path and query string (if present) and properly URL-encodes it
func buildEncodedRedirectURL(c *gin.Context) string {
	redirectURL := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		redirectURL += "?" + c.Request.URL.RawQuery
	}
	return url.QueryEscape(redirectURL)
}

// sendUnauthorizedJSON sends a JSON error response for unauthorized requests and aborts the context
func sendUnauthorizedJSON(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, models.AuthResponse{
		Success: false,
		Error:   &models.AuthError{Code: "AUTH_401", Message: message},
	})
	c.Abort()
}

// authConfig holds configuration for authentication middleware
type authConfig struct {
	cookieName        string
	storeTokenInCtx   bool
	allowRedirects    bool
	redirectLoginPath string
}

// validateAuthToken is a shared function that validates JWT tokens from cookies or headers
// It handles token extraction, validation, and error responses based on the provided config
func validateAuthToken(c *gin.Context, config authConfig) (string, *util.JWTClaims, bool) {
	// First try to get token from cookie
	token, err := c.Cookie(config.cookieName)
	if err != nil || token == "" {
		// Fallback to Authorization header for backward compatibility
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			// Handle missing token based on config
			if config.allowRedirects && !isAPIRequest(c) {
				// Redirect to login page for browser requests
				encodedRedirect := buildEncodedRedirectURL(c)
				c.Redirect(http.StatusTemporaryRedirect, config.redirectLoginPath+"?redirect="+encodedRedirect)
				c.Abort()
				return "", nil, false
			}
			// Return JSON error for API requests or when redirects are disabled
			sendUnauthorizedJSON(c, "Missing or invalid authentication token")
			return "", nil, false
		}
		token = strings.TrimSpace(authHeader[len("Bearer "):])
	}

	// Validate token
	claims, err := util.ValidateToken(token)
	if err != nil {
		// Set WWW-Authenticate header for client guidance
		if errors.Is(err, jwt.ErrTokenExpired) {
			c.Header("WWW-Authenticate", "Bearer error=\"invalid_token\", error_description=\"token expired\"")
		} else {
			c.Header("WWW-Authenticate", "Bearer error=\"invalid_token\", error_description=\"invalid token\"")
		}
		
		// Handle validation error based on config
		if config.allowRedirects && !isAPIRequest(c) {
			encodedRedirect := buildEncodedRedirectURL(c)
			c.Redirect(http.StatusTemporaryRedirect, config.redirectLoginPath+"?redirect="+encodedRedirect)
			c.Abort()
			return "", nil, false
		}
		sendUnauthorizedJSON(c, "Invalid or expired token")
		return "", nil, false
	}

	// Parse userID from claims
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		if config.allowRedirects && !isAPIRequest(c) {
			encodedRedirect := buildEncodedRedirectURL(c)
			c.Redirect(http.StatusTemporaryRedirect, config.redirectLoginPath+"?redirect="+encodedRedirect)
			c.Abort()
			return "", nil, false
		}
		sendUnauthorizedJSON(c, "Invalid user identity in token")
		return "", nil, false
	}

	// Set values in context
	if config.storeTokenInCtx {
		c.Set("token", token)
	}
	c.Set("claims", claims)
	c.Set("userID", userID)

	return token, claims, true
}

// SwaggerAuthRequired validates the JWT token from Swagger-specific HttpOnly cookie
// This keeps Swagger authentication separate from frontend authentication
func SwaggerAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := authConfig{
			cookieName:        "swagger_auth_token",
			storeTokenInCtx:   true,
			allowRedirects:    true,
			redirectLoginPath: "/auth/login-page",
		}
		
		_, _, ok := validateAuthToken(c, config)
		if !ok {
			return // Error already handled in validateAuthToken
		}
		
		c.Next()
	}
}

