package handler

import (
	"errors"
	"log"
	"net/http"
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

	// Use controller to create shortened URL
	urlRecord, err := h.urlController.GenerateShortCode(req.URL, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create shortened URL"})
		return
	}

	// Return the shortened URL
	shortened := "https://www.sniply.co.in/" + urlRecord.ShortCode

	c.JSON(http.StatusOK, gin.H{
		"shortened_url": shortened,
		"original_url":  req.URL,
		"short_code":    urlRecord.ShortCode,
	})
}

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
		ClickCount         int        `json:"click_count"`
		CreatedAt          time.Time  `json:"created_at"`
		UpdatedAt          time.Time  `json:"updated_at"`
		ExpiresAt          *time.Time `json:"expires_at"`
		TotalVisits        int64      `json:"total_visits"`
		UniqueVisitors     int64      `json:"unique_visitors"`
		LastVisitAt        *time.Time `json:"last_visit_at"`
		LastVisitUserAgent *string    `json:"last_visit_user_agent"`
	}

	response := make([]urlSummary, 0, len(summaries))
	for _, summary := range summaries {
		response = append(response, urlSummary{
			ShortCode:          summary.ShortCode,
			OriginalURL:        summary.OriginalURL,
			ClickCount:         summary.ClickCount,
			CreatedAt:          summary.CreatedAt,
			UpdatedAt:          summary.UpdatedAt,
			ExpiresAt:          summary.ExpiresAt,
			TotalVisits:        summary.TotalVisits,
			UniqueVisitors:     summary.UniqueVisitors,
			LastVisitAt:        summary.LastVisitAt,
			LastVisitUserAgent: summary.LastVisitUserAgent,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OK",
		"data":    response,
	})
}

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

func (h *Handler) RedirectURL(c *gin.Context) {
	code := c.Param("code")

	if strings.TrimSpace(code) == "" {
		log.Printf("event=redirect_error reason=missing_code")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing short code"})
		return
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

	if urlRecord.ExpiresAt != nil && urlRecord.ExpiresAt.Before(time.Now()) {
		log.Printf("event=redirect_error code=%s reason=expired", code)
		c.JSON(http.StatusGone, gin.H{"error": "Short URL has expired"})
		return
	}

	// Use controller methods to record the visit
	if err := h.urlController.IncrementClickCount(urlRecord.ID); err != nil {
		log.Printf("event=redirect_error code=%s reason=click_increment_failed err=%v", code, err)
	}

	if err := h.urlController.RecordVisit(urlRecord.ID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		log.Printf("event=redirect_error code=%s reason=visit_create_failed err=%v", code, err)
	}

	log.Printf("event=redirect_success code=%s url=%s", code, urlRecord.OriginalURL)
	c.Redirect(http.StatusFound, urlRecord.OriginalURL)
}

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

	c.JSON(http.StatusOK, gin.H{
		"short_code":            stats.ShortCode,
		"original_url":          stats.OriginalURL,
		"click_count":           stats.ClickCount,
		"total_visits":          stats.TotalVisits,
		"unique_visitors":       stats.UniqueVisitors,
		"last_visit_at":         stats.LastVisitAt,
		"last_visit_user_agent": stats.LastVisitUserAgent,
	})
}

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
func (h *Handler) GoogleAuth(c *gin.Context)     { h.auth.GoogleAuth(c) }
func (h *Handler) GoogleCallback(c *gin.Context) { h.auth.GoogleCallback(c) }
func (h *Handler) OAuthStatus(c *gin.Context)    { h.auth.OAuthStatus(c) }
func (h *Handler) TestJWT(c *gin.Context)        { h.auth.TestJWT(c) }
func (h *Handler) Me(c *gin.Context)             { h.auth.Me(c) }
