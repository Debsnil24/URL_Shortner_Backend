package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	urlController *controller.URLController
	auth          *AuthHandler
	emailService  *service.EmailService
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		urlController: controller.NewURLController(db),
		auth:          NewAuthHandler(db),
		emailService:  service.GetEmailService(), // Use singleton email service
	}
}

// GetUserIDFromContext extracts and validates userID from Gin context
// Returns the userID if found and valid, or an error if missing or invalid
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, errors.New("authentication required")
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid user identity")
	}

	return userID, nil
}

// TestHandler godoc
// @Summary      Test database connection
// @Description  Returns the count of tables in the database
// @Tags         test
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /api/test [get]
func (h *Handler) TestHandler(c *gin.Context) {
	var count int

	query := ` 
		SELECT COUNT(*) 
		FROM information_schema.tables 
		WHERE table_schema = 'public';
	`

	if err := h.urlController.DB.Raw(query).Scan(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": count,
	})
}

// ShortenURL godoc
// @Summary      Shorten a URL
// @Description  Creates a shortened URL from a long URL
// @Tags         urls
// @Accept       json
// @Produce      json
// @Param        request  body      models.ShortenURLRequest  true  "URL to shorten"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/shorten [post]
func (h *Handler) ShortenURL(c *gin.Context) {
	var req models.ShortenURLRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate URL format
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Validate that both preset and custom_expiration are not provided simultaneously
	if req.ExpirationPreset != "" && req.CustomExpiration != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot provide both expiration_preset and custom_expiration. Use only one."})
		return
	}

	// Use controller to create shortened URL
	urlRecord, err := h.urlController.GenerateShortCode(req.URL, userID, req.ExpirationPreset, req.CustomExpiration)
	if err != nil {
		// Check if it's a validation error (400) or calculation error (422)
		if strings.Contains(err.Error(), "must be between") ||
			strings.Contains(err.Error(), "cannot exceed") ||
			strings.Contains(err.Error(), "invalid expiration preset") ||
			strings.Contains(err.Error(), "must be in the future") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create shortened URL"})
		return
	}

	// Return the shortened URL
	shortened := "https://www.sniply.co.in/" + urlRecord.ShortCode

	response := gin.H{
		"shortened_url": shortened,
		"original_url":  req.URL,
		"short_code":    urlRecord.ShortCode,
	}

	// Include expires_at in response if it exists
	if urlRecord.ExpiresAt != nil {
		response["expires_at"] = urlRecord.ExpiresAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, response)
}

// ListURLs godoc
// @Summary      List user's URLs
// @Description  Returns all shortened URLs created by the authenticated user with statistics
// @Tags         urls
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/urls [get]
func (h *Handler) ListURLs(c *gin.Context) {
	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Use controller to fetch URLs with statistics
	summaries, err := h.urlController.ListURLsByUser(userID)
	if err != nil {
		log.Printf("event=list_urls_error user_id=%s reason=query_failed err=%v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URLs"})
		return
	}

	// Convert controller URLSummary to response format
	type urlSummary struct {
		ShortCode          string     `json:"short_code"`
		OriginalURL        string     `json:"original_url"`
		Status             string     `json:"status"`
		ClickCount         int        `json:"click_count"`
		CreatedAt          time.Time  `json:"created_at"`
		UpdatedAt          time.Time  `json:"updated_at"`
		ExpiresAt          *time.Time `json:"expires_at"`
		TotalVisits        int64      `json:"total_visits"`
		UniqueVisitors     int64      `json:"unique_visitors"`
		LastVisitAt        *time.Time `json:"last_visit_at"`
		LastVisitUserAgent *string    `json:"last_visit_user_agent"`
		QRCodeAvailable    bool       `json:"qr_code_available"`
		QRCodeSize         int        `json:"qr_code_size,omitempty"`
		QRCodeFormat       string     `json:"qr_code_format,omitempty"`
		QRCodeGeneratedAt  *time.Time `json:"qr_code_generated_at,omitempty"`
		QRCodeURL          string     `json:"qr_code_url,omitempty"` // Direct URL to fetch QR code image
	}

	response := make([]urlSummary, 0, len(summaries))
	for _, summary := range summaries {
		// Build QR code URL if QR code is available
		qrCodeURL := ""
		if summary.QRCodeAvailable {
			qrCodeURL = fmt.Sprintf("/api/urls/%s/qr", summary.ShortCode)
		}

		response = append(response, urlSummary{
			ShortCode:          summary.ShortCode,
			OriginalURL:        summary.OriginalURL,
			Status:             summary.Status,
			ClickCount:         summary.ClickCount,
			CreatedAt:          summary.CreatedAt,
			UpdatedAt:          summary.UpdatedAt,
			ExpiresAt:          summary.ExpiresAt,
			TotalVisits:        summary.TotalVisits,
			UniqueVisitors:     summary.UniqueVisitors,
			LastVisitAt:        summary.LastVisitAt,
			LastVisitUserAgent: summary.LastVisitUserAgent,
			QRCodeAvailable:    summary.QRCodeAvailable,
			QRCodeSize:         summary.QRCodeSize,
			QRCodeFormat:       summary.QRCodeFormat,
			QRCodeGeneratedAt:  summary.QRCodeGeneratedAt,
			QRCodeURL:          qrCodeURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OK",
		"data":    response,
	})
}

// DeleteURL godoc
// @Summary      Delete a URL
// @Description  Deletes a shortened URL by its code (only if owned by the authenticated user)
// @Tags         urls
// @Accept       json
// @Produce      json
// @Param        code  path      string  true  "Short code of the URL"
// @Success      200   {object}  map[string]interface{}
// @Failure      401   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/delete/{code} [delete]
func (h *Handler) DeleteURL(c *gin.Context) {
	code := c.Param("code")

	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Use controller to delete URL (includes ownership check)
	err = h.urlController.DeleteURL(code, userID)
	if err != nil {
		if err.Error() == "URL not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		if err.Error() == "permission denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this URL"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "URL deleted successfully"})
}

// UpdateURLStatus godoc
// @Summary      Update URL status
// @Description  Updates the status of a short link (pause or resume). Only updates the status field, does not affect URL or expiration.
// @Tags         urls
// @Accept       json
// @Produce      json
// @Param        code  path      string  true  "Short code of the URL"
// @Param        request  body      models.UpdateStatusRequest  true  "Status update request"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Failure      403      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/urls/{code}/status [patch]
func (h *Handler) UpdateURLStatus(c *gin.Context) {
	code := c.Param("code")

	var req models.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	urlRecord, err := h.urlController.UpdateURLStatus(code, userID, req.Status)
	if err != nil {
		if err.Error() == "URL not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		if err.Error() == "permission denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to update this URL"})
			return
		}
		if err.Error() == "status must be either 'active' or 'paused'" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value. Status must be either 'active' or 'paused'"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update URL status"})
		return
	}

	// Build response data
	data := gin.H{
		"short_code":   urlRecord.ShortCode,
		"original_url": urlRecord.OriginalURL,
		"status":       urlRecord.Status,
		"click_count":  urlRecord.ClickCount,
		"created_at":   urlRecord.CreatedAt,
		"updated_at":   urlRecord.UpdatedAt,
	}

	// Include expires_at if it exists
	if urlRecord.ExpiresAt != nil {
		data["expires_at"] = urlRecord.ExpiresAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "URL status updated successfully",
		"data":    data,
	})
}

// UpdateURL godoc
// @Summary      Update a short URL
// @Description  Updates the original URL and/or expiration date of a shortened URL (only if owned by the authenticated user)
// @Tags         urls
// @Accept       json
// @Produce      json
// @Param        code  path      string  true  "Short code of the URL"
// @Param        request  body      models.UpdateURLRequest  true  "Update request"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Failure      403      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Failure      410      {object}  map[string]interface{}
// @Failure      422      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/urls/{code} [patch]
func (h *Handler) UpdateURL(c *gin.Context) {
	code := c.Param("code")

	var req models.UpdateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	urlRecord, err := h.urlController.UpdateURL(code, userID, &req)
	if err != nil {
		if err.Error() == "URL not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		if err.Error() == "permission denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to update this URL"})
			return
		}
		if err.Error() == "cannot update URL of expired link. Update expiration to reactivate it" {
			c.JSON(http.StatusGone, gin.H{"error": "Cannot update URL of expired link. Update expiration to reactivate it."})
			return
		}
		if strings.Contains(err.Error(), "at least one field") ||
			strings.Contains(err.Error(), "cannot provide both") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "must be between") ||
			strings.Contains(err.Error(), "cannot exceed") ||
			strings.Contains(err.Error(), "invalid expiration preset") ||
			strings.Contains(err.Error(), "must be in the future") ||
			strings.Contains(err.Error(), "invalid URL format") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update URL"})
		return
	}

	// Build response data
	data := gin.H{
		"short_code":   urlRecord.ShortCode,
		"original_url": urlRecord.OriginalURL,
		"status":       urlRecord.Status,
		"click_count":  urlRecord.ClickCount,
		"created_at":   urlRecord.CreatedAt,
		"updated_at":   urlRecord.UpdatedAt,
	}

	// Include expires_at if it exists
	if urlRecord.ExpiresAt != nil {
		data["expires_at"] = urlRecord.ExpiresAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "URL updated successfully",
		"data":    data,
	})
}

// RedirectURL godoc
// @Summary      Redirect to original URL
// @Description  Redirects to the original URL associated with the short code. Supports both GET and HEAD methods. GET requests count as clicks and redirect, while HEAD requests only check status and do not count as clicks.
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        code  path      string  true  "Short code"
// @Success      200   {string}  string  "HEAD request - Status check only (no click counted)"
// @Success      302   {string}  string  "GET request - Redirect (click counted)"
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      410   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /{code} [get]
// @Router       /{code} [head]
func (h *Handler) RedirectURL(c *gin.Context) {
	code := c.Param("code")

	// Trim whitespace and validate
	code = strings.TrimSpace(code)
	if code == "" {
		log.Printf("event=redirect_error reason=missing_code")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing short code"})
		return
	}

	// Exclude reserved paths that should not be treated as short codes
	reservedPaths := []string{"swagger", "api", "auth", "favicon.ico", "robots.txt"}
	for _, reserved := range reservedPaths {
		if code == reserved {
			c.JSON(http.StatusNotFound, gin.H{"error": "Short URL not found"})
			return
		}
	}

	// Use controller to get URL by code
	urlRecord, err := h.urlController.GetURLByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("event=redirect_error code=%s reason=not_found", code)
			c.JSON(http.StatusNotFound, gin.H{"error": "Short URL not found"})
			return
		}
		log.Printf("event=redirect_error code=%s err=%v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve short URL"})
		return
	}

	// Check status first - if paused, return 410 (applies to both GET and HEAD)
	if urlRecord.Status == "paused" {
		log.Printf("event=redirect_error code=%s reason=paused", code)
		c.JSON(http.StatusGone, gin.H{"error": "Link is paused"})
		return
	}

	// Check expiration - improved time comparison (handles exact time matches)
	now := time.Now()
	if urlRecord.ExpiresAt != nil {
		if urlRecord.ExpiresAt.Before(now) || urlRecord.ExpiresAt.Equal(now) {
			log.Printf("event=redirect_error code=%s reason=expired", code)
			c.JSON(http.StatusGone, gin.H{"error": "Link has expired"})
			return
		}
	}

	// Handle HEAD requests - just return status, don't count as click or redirect
	// HEAD is used for link status checks by frontend
	if c.Request.Method == "HEAD" {
		c.Header("Location", urlRecord.OriginalURL)
		c.Status(http.StatusOK)
		return
	}

	// Only count GET requests as clicks (not HEAD requests)
	// Use atomic method to record visit and increment click count together
	// This ensures both operations succeed or fail together, preventing data inconsistency
	if err := h.urlController.RecordVisitAndIncrement(urlRecord.ID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		log.Printf("event=redirect_error code=%s reason=visit_record_failed err=%v", code, err)
		// Continue with redirect even if recording fails - don't block user experience
	}

	log.Printf("event=redirect_success code=%s url=%s", code, urlRecord.OriginalURL)
	c.Redirect(http.StatusFound, urlRecord.OriginalURL)
}

// GetURLStats godoc
// @Summary      Get URL statistics
// @Description  Returns detailed statistics for a shortened URL (only if owned by the authenticated user)
// @Tags         urls
// @Accept       json
// @Produce      json
// @Param        code  path      string  true  "Short code of the URL"
// @Success      200   {object}  map[string]interface{}
// @Failure      401   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/urls/{code}/stats [get]
func (h *Handler) GetURLStats(c *gin.Context) {
	code := c.Param("code")

	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Use controller to get URL statistics (includes ownership check)
	stats, err := h.urlController.GetURLStats(code, userID)
	if err != nil {
		if err.Error() == "URL not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		if err.Error() == "permission denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this URL"})
			return
		}
		log.Printf("event=url_stats_error code=%s err=%v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch statistics"})
		return
	}

	// Get URL record to access expires_at
	urlRecord, err := h.urlController.GetURLByCode(code)
	if err != nil {
		// If we can't get URL record, still return stats but without expires_at
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"short_code":            stats.ShortCode,
				"original_url":          stats.OriginalURL,
				"status":                stats.Status,
				"click_count":           stats.ClickCount,
				"total_visits":          stats.TotalVisits,
				"unique_visitors":       stats.UniqueVisitors,
				"last_visit_at":         stats.LastVisitAt,
				"last_visit_user_agent": stats.LastVisitUserAgent,
			},
		})
		return
	}

	responseData := gin.H{
		"short_code":            stats.ShortCode,
		"original_url":          stats.OriginalURL,
		"status":                stats.Status,
		"click_count":           stats.ClickCount,
		"total_visits":          stats.TotalVisits,
		"unique_visitors":       stats.UniqueVisitors,
		"last_visit_at":         stats.LastVisitAt,
		"last_visit_user_agent": stats.LastVisitUserAgent,
	}

	// Include expires_at if it exists
	if urlRecord.ExpiresAt != nil {
		responseData["expires_at"] = urlRecord.ExpiresAt.Format(time.RFC3339)
	}

	// Include QR code metadata for frontend integration
	if urlRecord.QRCodeGeneratedAt != nil && urlRecord.QRCodeSize > 0 {
		responseData["qr_code_available"] = true
		responseData["qr_code_size"] = urlRecord.QRCodeSize
		responseData["qr_code_format"] = urlRecord.QRCodeFormat
		responseData["qr_code_generated_at"] = urlRecord.QRCodeGeneratedAt.Format(time.RFC3339)
		responseData["qr_code_url"] = fmt.Sprintf("/api/urls/%s/qr", code)
	} else {
		responseData["qr_code_available"] = false
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responseData,
	})
}

// GetQRCode godoc
// @Summary      Get QR code image
// @Description  Returns the QR code image for a short link. If QR code doesn't exist, it will be generated automatically with default size (256px). Use ?download=true to force download instead of inline display.
// @Tags         urls
// @Accept       json
// @Produce      image/png
// @Param        code     path      string  true   "Short code of the URL"
// @Param        download query     bool    false  "Force download (true) or inline display (false, default)"
// @Success      200      {file}    binary  "QR code PNG image"
// @Failure      401      {object}  map[string]interface{}
// @Failure      403      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/urls/{code}/qr [get]
func (h *Handler) GetQRCode(c *gin.Context) {
	code := c.Param("code")

	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Use controller to get QR code (includes ownership check and lazy generation)
	qrCodeBytes, _, format, err := h.urlController.GetQRCode(code, userID)
	if err != nil {
		if err.Error() == "URL not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		if err.Error() == "permission denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to access this URL"})
			return
		}
		log.Printf("event=get_qr_code_error code=%s err=%v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get QR code"})
		return
	}

	// Set appropriate content type based on format
	contentType := "image/png"
	if format == "svg" {
		contentType = "image/svg+xml"
	}

	// Check if download parameter is set
	download := c.Query("download") == "true" || c.Query("download") == "1"

	// Set headers
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", len(qrCodeBytes)))

	// Set Content-Disposition header based on download parameter
	if download {
		// Force download with filename
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"qr-%s.%s\"", code, format))
	} else {
		// Inline display (default)
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"qr-%s.%s\"", code, format))
	}

	// Cache QR codes for 24 hours (they don't change unless regenerated)
	c.Header("Cache-Control", "public, max-age=86400")

	// Return image
	c.Data(http.StatusOK, contentType, qrCodeBytes)
}

// GenerateQRCode godoc
// @Summary      Generate or regenerate QR code
// @Description  Generates or regenerates a QR code for a short link with the specified size. If QR code already exists, it will be regenerated.
// @Tags         urls
// @Accept       json
// @Produce      json
// @Param        code  path      string  true  "Short code of the URL"
// @Param        request  body      object  false  "QR code generation request"
// @Param        size  query     int     false  "QR code size in pixels (256, 512, or 1024). Default: 256"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Failure      403      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/urls/{code}/qr [post]
func (h *Handler) GenerateQRCode(c *gin.Context) {
	code := c.Param("code")

	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Get size from query parameter or request body, default to 256
	size := 256
	if sizeParam := c.Query("size"); sizeParam != "" {
		parsedSize, err := strconv.Atoi(sizeParam)
		if err == nil {
			size = parsedSize
		}
	} else {
		// Try to get from request body
		var req struct {
			Size int `json:"size"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.Size > 0 {
			size = req.Size
		}
	}

	// Use controller to generate QR code (includes ownership check)
	urlRecord, err := h.urlController.GenerateQRCode(code, userID, size)
	if err != nil {
		if err.Error() == "URL not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		if err.Error() == "permission denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to access this URL"})
			return
		}
		if strings.Contains(err.Error(), "invalid QR code size") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("event=generate_qr_code_error code=%s err=%v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}

	// Build response data
	data := gin.H{
		"short_code":           urlRecord.ShortCode,
		"original_url":         urlRecord.OriginalURL,
		"qr_code_size":         urlRecord.QRCodeSize,
		"qr_code_format":       urlRecord.QRCodeFormat,
		"qr_code_generated_at": nil,
	}

	// Include qr_code_generated_at if it exists
	if urlRecord.QRCodeGeneratedAt != nil {
		data["qr_code_generated_at"] = urlRecord.QRCodeGeneratedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "QR code generated successfully",
		"data":    data,
	})
}

// SubmitSupport godoc
// @Summary      Submit support request
// @Description  Submits a support request with rate limiting
// @Tags         support
// @Accept       json
// @Produce      json
// @Param        request  body      models.SupportRequest  true  "Support request details"
// @Success      200      {object}  models.SupportResponse
// @Failure      400      {object}  models.SupportResponse
// @Failure      429      {object}  map[string]interface{}
// @Router       /api/support [post]
func (h *Handler) SubmitSupport(c *gin.Context) {
	var req models.SupportRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.SupportResponse{
			Success: false,
			Message: "Invalid request data",
			Error: &models.AuthError{
				Code:    "VALIDATION_ERROR",
				Message: "Please check your input and try again.",
			},
		})
		return
	}

	// Sanitize inputs: trim whitespace
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Message = strings.TrimSpace(req.Message)

	// Additional validation after trimming
	if req.Name == "" || req.Email == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, models.SupportResponse{
			Success: false,
			Message: "All fields are required",
			Error: &models.AuthError{
				Code:    "VALIDATION_ERROR",
				Message: "Name, email, and message cannot be empty.",
			},
		})
		return
	}

	// Send email asynchronously - don't block HTTP response
	// This improves user experience as they get immediate feedback
	h.emailService.SendSupportEmailAsync(req.Name, req.Email, req.Message)

	log.Printf("event=support_request_submitted name=%s email=%s ip=%s", req.Name, req.Email, c.ClientIP())
	c.JSON(http.StatusOK, models.SupportResponse{
		Success: true,
		Message: "Support request submitted successfully. We'll get back to you soon!",
	})
}

// Auth proxy methods for route wiring convenience
func (h *Handler) Register(c *gin.Context)       { h.auth.Register(c) }
func (h *Handler) Login(c *gin.Context)          { h.auth.Login(c) }
func (h *Handler) Logout(c *gin.Context)         { h.auth.Logout(c) }
func (h *Handler) SwaggerLogout(c *gin.Context)  { h.auth.SwaggerLogout(c) }
func (h *Handler) GoogleAuth(c *gin.Context)     { h.auth.GoogleAuth(c) }
func (h *Handler) GoogleCallback(c *gin.Context) { h.auth.GoogleCallback(c) }
func (h *Handler) OAuthStatus(c *gin.Context)    { h.auth.OAuthStatus(c) }
func (h *Handler) TestJWT(c *gin.Context)        { h.auth.TestJWT(c) }
func (h *Handler) Me(c *gin.Context)             { h.auth.Me(c) }
