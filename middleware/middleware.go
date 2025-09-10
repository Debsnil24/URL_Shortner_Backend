package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
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

		c.Set("claims", claims)
		c.Next()
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
