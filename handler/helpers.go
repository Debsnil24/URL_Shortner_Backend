package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
)

// setNoCacheHeaders sets cache-control headers to prevent caching
// Used for redirect responses and error responses that need to be fresh
func setNoCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// handleControllerError handles controller errors and returns appropriate HTTP responses
// Returns true if the error was handled, false otherwise
func handleControllerError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	// Check for specific controller errors using errors.Is()
	if errors.Is(err, controller.ErrURLNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return true
	}

	if errors.Is(err, controller.ErrPermissionDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to perform this action"})
		return true
	}

	if errors.Is(err, controller.ErrInvalidStatus) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value. Status must be either 'active' or 'paused'"})
		return true
	}

	if errors.Is(err, controller.ErrExpiredLinkUpdate) {
		c.JSON(http.StatusGone, gin.H{"error": "Cannot update URL of expired link. Update expiration to reactivate it."})
		return true
	}

	if errors.Is(err, controller.ErrNoFieldsProvided) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return true
	}

	if errors.Is(err, controller.ErrBothExpirationProvided) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return true
	}

	if errors.Is(err, controller.ErrInvalidQRCodeSize) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return true
	}

	// Check for validation errors (string-based for now, as they come from validation library)
	errMsg := err.Error()
	if strings.Contains(errMsg, "must be between") ||
		strings.Contains(errMsg, "cannot exceed") ||
		strings.Contains(errMsg, "invalid expiration preset") ||
		strings.Contains(errMsg, "must be in the future") ||
		strings.Contains(errMsg, "invalid URL format") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": errMsg})
		return true
	}

	// Error not handled by this function
	return false
}

// IsStatusPaused checks if a status is paused (case-insensitive)
// Delegates to util.IsStatusPaused for consistency
func IsStatusPaused(status string) bool {
	return util.IsStatusPaused(status)
}

