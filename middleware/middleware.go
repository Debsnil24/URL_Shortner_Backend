package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthRequired validates the JWT token from HttpOnly cookie and sets claims in context
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First try to get token from HttpOnly cookie
		token, err := c.Cookie("auth_token")
		if err != nil || token == "" {
			// Fallback to Authorization header for backward compatibility
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Missing or invalid authentication token"}})
				c.Abort()
				return
			}
			token = strings.TrimSpace(authHeader[len("Bearer "):])
		}

		claims, err := util.ValidateToken(token)
		if err != nil {
			// Help clients decide to logout by signaling token status
			if errors.Is(err, jwt.ErrTokenExpired) {
				c.Header("WWW-Authenticate", "Bearer error=\"invalid_token\", error_description=\"token expired\"")
			} else {
				c.Header("WWW-Authenticate", "Bearer error=\"invalid_token\", error_description=\"invalid token\"")
			}
			c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Invalid or expired token"}})
			c.Abort()
			return
		}

		// Parse userID from claims and set in context for easy access
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Invalid user identity in token"}})
			c.Abort()
			return
		}

		// Set both claims and parsed userID in context
		c.Set("claims", claims)
		c.Set("userID", userID)
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
