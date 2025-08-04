package handler

import (
	"net/http"
	"strings"

	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	urlController *controller.URLController
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		urlController: controller.NewURLController(db),
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

	// Use controller to create shortened URL
	urlRecord, err := h.urlController.GenerateShortCode(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create shortened URL"})
		return
	}

	// Return the shortened URL
	shortened := "http://localhost:8080/" + urlRecord.ShortCode

	c.JSON(http.StatusOK, gin.H{
		"shortened_url": shortened,
		"original_url":  req.URL,
		"short_code":    urlRecord.ShortCode,
	})
}


func (h *Handler) DeleteURL(c *gin.Context) {
	code := c.Param("code")

	if err := h.urlController.DeleteURL(code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "URL deleted successfully"})
}