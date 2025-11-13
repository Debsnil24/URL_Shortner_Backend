package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	urlController *controller.URLController
	auth          *AuthHandler
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		urlController: controller.NewURLController(db),
		auth:          NewAuthHandler(db),
	}
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
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user identity"})
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
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user identity"})
		return
	}

	var urls []models.URL
	if err := h.urlController.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&urls).Error; err != nil {
		log.Printf("event=list_urls_error user_id=%s reason=query_failed err=%v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URLs"})
		return
	}

	type urlSummary struct {
		ShortCode          string     `json:"short_code"`
		OriginalURL        string     `json:"original_url"`
		ClickCount         int        `json:"click_count"`
		CreatedAt          time.Time  `json:"created_at"`
		UpdatedAt          time.Time  `json:"updated_at"`
		ExpiresAt          *time.Time `json:"expires_at"`
		TotalVisits        int64      `json:"total_visits"`
		LastVisitAt        *time.Time `json:"last_visit_at"`
		LastVisitUserAgent *string    `json:"last_visit_user_agent"`
	}

	response := make([]urlSummary, 0, len(urls))

	for _, urlRecord := range urls {
		var visitCount int64
		if err := h.urlController.DB.Model(&models.URLVisit{}).Where("url_id = ?", urlRecord.ID).Count(&visitCount).Error; err != nil {
			log.Printf("event=list_urls_error short_code=%s reason=count_failed err=%v", urlRecord.ShortCode, err)
		}

		var latestVisit models.URLVisit
		var lastVisitAt *time.Time
		var lastVisitUserAgent *string

		if err := h.urlController.DB.Where("url_id = ?", urlRecord.ID).Order("created_at DESC").First(&latestVisit).Error; err == nil {
			lastVisitAt = &latestVisit.CreatedAt
			ua := latestVisit.UserAgent
			lastVisitUserAgent = &ua
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("event=list_urls_error short_code=%s reason=latest_visit_failed err=%v", urlRecord.ShortCode, err)
		}

		response = append(response, urlSummary{
			ShortCode:          urlRecord.ShortCode,
			OriginalURL:        urlRecord.OriginalURL,
			ClickCount:         urlRecord.ClickCount,
			CreatedAt:          urlRecord.CreatedAt,
			UpdatedAt:          urlRecord.UpdatedAt,
			ExpiresAt:          urlRecord.ExpiresAt,
			TotalVisits:        visitCount,
			LastVisitAt:        lastVisitAt,
			LastVisitUserAgent: lastVisitUserAgent,
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
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user identity"})
		return
	}

	var urlRecord models.URL
	if err := h.urlController.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lookup URL"})
		return
	}

	if urlRecord.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this URL"})
		return
	}

	if err := h.urlController.DB.Delete(&urlRecord).Error; err != nil {
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

	var urlRecord models.URL
	if err := h.urlController.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
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

	if err := h.urlController.DB.Model(&urlRecord).Where("id = ?", urlRecord.ID).UpdateColumn("click_count", gorm.Expr("click_count + ?", 1)).Error; err != nil {
		log.Printf("event=redirect_error code=%s reason=click_increment_failed err=%v", code, err)
	}

	visit := models.URLVisit{
		URLID:     urlRecord.ID,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}
	if err := h.urlController.DB.Create(&visit).Error; err != nil {
		log.Printf("event=redirect_error code=%s reason=visit_create_failed err=%v", code, err)
	}

	log.Printf("event=redirect_success code=%s url=%s", code, urlRecord.OriginalURL)
	c.Redirect(http.StatusFound, urlRecord.OriginalURL)
}

func (h *Handler) GetURLStats(c *gin.Context) {
	code := c.Param("code")

	// Get userID from context (set by AuthRequired middleware)
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user identity"})
		return
	}

	var urlRecord models.URL
	if err := h.urlController.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		log.Printf("event=url_stats_error code=%s err=%v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URL"})
		return
	}

	if urlRecord.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this URL"})
		return
	}

	var visitCount int64
	if err := h.urlController.DB.Model(&models.URLVisit{}).Where("url_id = ?", urlRecord.ID).Count(&visitCount).Error; err != nil {
		log.Printf("event=url_stats_error code=%s reason=count_failed err=%v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate statistics"})
		return
	}

	var latestVisit models.URLVisit
	latestVisitTime := (*time.Time)(nil)
	if err := h.urlController.DB.Where("url_id = ?", urlRecord.ID).Order("created_at DESC").First(&latestVisit).Error; err == nil {
		latestVisitTime = &latestVisit.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("event=url_stats_error code=%s reason=latest_visit_failed err=%v", code, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"short_code":            urlRecord.ShortCode,
		"original_url":          urlRecord.OriginalURL,
		"click_count":           urlRecord.ClickCount,
		"total_visits":          visitCount,
		"last_visit_at":         latestVisitTime,
		"last_visit_user_agent": latestVisit.UserAgent,
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
