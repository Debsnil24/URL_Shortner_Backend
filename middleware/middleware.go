package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthRequired validates the JWT token from HttpOnly cookie and sets claims in context
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Special handling for Swagger UI access - redirect to login page
		if strings.HasPrefix(c.Request.URL.Path, "/swagger") {
			encodedRedirect := buildEncodedRedirectURL(c)
			c.Redirect(http.StatusTemporaryRedirect, "/auth/login-page?redirect="+encodedRedirect)
			c.Abort()
			return
		}

		config := authConfig{
			cookieName:        "auth_token",
			storeTokenInCtx:   false,
			allowRedirects:    false, // Always return JSON errors
			redirectLoginPath: "/auth/login-page",
		}
		
		_, _, ok := validateAuthToken(c, config)
		if !ok {
			return // Error already handled in validateAuthToken
		}
		
		c.Next()
	}
}

// OptionalAuth validates JWT token from either QR token (Authorization header) or cookie
// Sets userID in context if either authentication method succeeds
// Does not abort if authentication fails - allows handler to decide
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First try QR token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			tokenString := strings.TrimSpace(authHeader[len("Bearer "):])
			qrClaims, err := util.ValidateQRToken(tokenString)
			if err == nil {
				// Valid QR token - set userID in context
				userID, parseErr := uuid.Parse(qrClaims.UserID)
				if parseErr == nil {
					c.Set("userID", userID)
					c.Set("userEmail", qrClaims.Email)
					c.Set("authMethod", "qr_token")
					c.Next()
					return
				}
			}
			// QR token invalid, fall through to cookie auth
		}

		// Fallback to cookie-based authentication
		config := authConfig{
			cookieName:        "auth_token",
			storeTokenInCtx:   false,
			allowRedirects:    false,
			redirectLoginPath: "/auth/login-page",
		}
		
		_, _, ok := validateAuthToken(c, config)
		if ok {
			c.Set("authMethod", "cookie")
			// userID already set by validateAuthToken
		}
		// Don't abort - let handler decide what to do
		
		c.Next()
	}
}

// RateLimiter stores request timestamps per IP address
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	maxReqs  int
	window   time.Duration
	cleanup  *time.Ticker
}

// NewRateLimiter creates a new rate limiter
// maxRequests: maximum number of requests allowed
// window: time window for rate limiting (e.g., 1 minute)
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		maxReqs:  maxRequests,
		window:   window,
		cleanup:  time.NewTicker(5 * time.Minute), // Cleanup old entries every 5 minutes
	}

	// Start cleanup goroutine
	go func() {
		for range rl.cleanup.C {
			rl.cleanupOldEntries()
		}
	}()

	return rl
}

// cleanupOldEntries removes entries older than the rate limit window
func (rl *RateLimiter) cleanupOldEntries() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for ip, times := range rl.requests {
		// Filter out old timestamps
		validTimes := make([]time.Time, 0)
		for _, t := range times {
			if t.After(cutoff) {
				validTimes = append(validTimes, t)
			}
		}

		if len(validTimes) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = validTimes
		}
	}
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get existing requests for this IP
	times := rl.requests[ip]

	// Remove old requests outside the window
	validTimes := make([]time.Time, 0)
	for _, t := range times {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}

	// Check if we've exceeded the limit
	if len(validTimes) >= rl.maxReqs {
		return false
	}

	// Add current request
	validTimes = append(validTimes, now)
	rl.requests[ip] = validTimes

	return true
}

// Global rate limiter instance for support endpoint
// 5 requests per 15 minutes per IP
var supportRateLimiter = NewRateLimiter(5, 15*time.Minute)

// RateLimit middleware limits requests per IP address
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !supportRateLimiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, models.SupportResponse{
				Success: false,
				Message: "Too many requests. Please try again later.",
				Error: &models.AuthError{
					Code:    "RATE_LIMIT_EXCEEDED",
					Message: "You have exceeded the rate limit. Please wait before submitting another request.",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestTimeout cancels the request if the handler chain exceeds the given timeout
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		finished := make(chan struct{})
		go func() {
			c.Next()
			close(finished)
		}()

		select {
		case <-finished:
			return
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded && !c.IsAborted() {
				c.AbortWithStatusJSON(http.StatusGatewayTimeout, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_408", Message: "Request timed out"}})
			}
			return
		}
	}
}

// SwaggerTokenEndpoint godoc
// @Summary      Get Swagger authentication token
// @Description  Returns the JWT token from the Swagger authentication cookie. This endpoint is used internally by Swagger UI to automatically set the Bearer token. Requires Swagger-specific authentication (swagger_auth_token cookie).
// @Tags         swagger
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string  "Token object with 'token' field"
// @Failure      401  {object}  models.AuthResponse
// @Failure      500  {object}  models.AuthResponse
// @Security     BearerAuth
// @Router       /api/swagger-token [get]
// SwaggerTokenEndpoint returns the JWT token from context (already validated by SwaggerAuthRequired middleware)
// This endpoint is only accessible to authenticated users and is used by Swagger UI
// to automatically set the Bearer token
func SwaggerTokenEndpoint() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from context (set by SwaggerAuthRequired middleware after validation)
		token, exists := c.Get("token")
		if !exists {
			// This should never happen if SwaggerAuthRequired is properly configured
			c.JSON(http.StatusInternalServerError, models.AuthResponse{
				Success: false,
				Error:   &models.AuthError{Code: "AUTH_500", Message: "Token not found in context"},
			})
			c.Abort()
			return
		}

		tokenStr, ok := token.(string)
		if !ok || tokenStr == "" {
			c.JSON(http.StatusInternalServerError, models.AuthResponse{
				Success: false,
				Error:   &models.AuthError{Code: "AUTH_500", Message: "Invalid token in context"},
			})
			c.Abort()
			return
		}

		// Return token in response (only for Swagger UI, authenticated users only)
		c.JSON(http.StatusOK, gin.H{
			"token": tokenStr,
		})
	}
}
