package controller

import (
	"errors"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// URLSummary represents a URL with its visit statistics
type URLSummary struct {
	ShortCode          string
	OriginalURL        string
	ClickCount         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExpiresAt          *time.Time
	TotalVisits        int64
	LastVisitAt        *time.Time
	LastVisitUserAgent *string
}

// URLStats represents statistics for a URL
type URLStats struct {
	ShortCode          string
	OriginalURL        string
	ClickCount         int
	TotalVisits        int64
	LastVisitAt        *time.Time
	LastVisitUserAgent string
}

type URLController struct {
	DB *gorm.DB
}

// NewURLController creates a new URL controller instance
func NewURLController(db *gorm.DB) *URLController {
	return &URLController{
		DB: db,
	}
}

func (c *URLController) GenerateShortCode(originalURL string, userID uuid.UUID) (*models.URL, error) {
	for {
		// Generate short code using util function
		code := util.GenerateShortCode()

		// Check if this code already exists in database
		var existingURL models.URL
		if err := c.DB.Where("short_code = ?", code).First(&existingURL).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				// Database error, abort and surface error
				return nil, err
			}

			createdAt := time.Now()
			expiresAt := createdAt.AddDate(5, 0, 0)

			urlRecord := models.URL{
				ShortCode:   code,
				OriginalURL: originalURL,
				ClickCount:  0,
				UserID:      userID,
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
				ExpiresAt:   &expiresAt,
			}

			if err := c.DB.Create(&urlRecord).Error; err != nil {
				return nil, err
			}

			return &urlRecord, nil
		}

		// Code exists, generate a new one
	}
}

// GetURLByCode retrieves a URL by its short code
func (c *URLController) GetURLByCode(code string) (*models.URL, error) {
	var urlRecord models.URL
	if err := c.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		return nil, err
	}
	return &urlRecord, nil
}

// ListURLsByUser retrieves all URLs for a user with visit statistics
func (c *URLController) ListURLsByUser(userID uuid.UUID) ([]URLSummary, error) {
	var urls []models.URL
	if err := c.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&urls).Error; err != nil {
		return nil, err
	}

	summaries := make([]URLSummary, 0, len(urls))
	for _, urlRecord := range urls {
		visitCount, err := c.GetVisitCount(urlRecord.ID)
		if err != nil {
			visitCount = 0 // Continue even if count fails
		}

		latestVisit, err := c.GetLatestVisit(urlRecord.ID)
		var lastVisitAt *time.Time
		var lastVisitUserAgent *string
		if err == nil && latestVisit != nil {
			lastVisitAt = &latestVisit.CreatedAt
			ua := latestVisit.UserAgent
			lastVisitUserAgent = &ua
		}

		summaries = append(summaries, URLSummary{
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

	return summaries, nil
}

// DeleteURL deletes a URL if it belongs to the specified user
func (c *URLController) DeleteURL(code string, userID uuid.UUID) error {
	var urlRecord models.URL
	if err := c.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("URL not found")
		}
		return err
	}

	if urlRecord.UserID != userID {
		return errors.New("permission denied")
	}

	if err := c.DB.Delete(&urlRecord).Error; err != nil {
		return err
	}

	return nil
}

// IncrementClickCount increments the click count for a URL
func (c *URLController) IncrementClickCount(urlID uint) error {
	return c.DB.Model(&models.URL{}).Where("id = ?", urlID).UpdateColumn("click_count", gorm.Expr("click_count + ?", 1)).Error
}

// RecordVisit creates a new visit record for a URL
func (c *URLController) RecordVisit(urlID uint, ipAddress, userAgent string) error {
	visit := models.URLVisit{
		URLID:     urlID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}
	return c.DB.Create(&visit).Error
}

// GetVisitCount returns the total number of visits for a URL
func (c *URLController) GetVisitCount(urlID uint) (int64, error) {
	var count int64
	if err := c.DB.Model(&models.URLVisit{}).Where("url_id = ?", urlID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetLatestVisit returns the most recent visit for a URL
func (c *URLController) GetLatestVisit(urlID uint) (*models.URLVisit, error) {
	var visit models.URLVisit
	if err := c.DB.Where("url_id = ?", urlID).Order("created_at DESC").First(&visit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No visits yet, not an error
		}
		return nil, err
	}
	return &visit, nil
}

// GetURLStats retrieves statistics for a URL if it belongs to the specified user
func (c *URLController) GetURLStats(code string, userID uuid.UUID) (*URLStats, error) {
	urlRecord, err := c.GetURLByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("URL not found")
		}
		return nil, err
	}

	if urlRecord.UserID != userID {
		return nil, errors.New("permission denied")
	}

	visitCount, err := c.GetVisitCount(urlRecord.ID)
	if err != nil {
		return nil, err
	}

	latestVisit, err := c.GetLatestVisit(urlRecord.ID)
	var lastVisitAt *time.Time
	var lastVisitUserAgent string
	if err == nil && latestVisit != nil {
		lastVisitAt = &latestVisit.CreatedAt
		lastVisitUserAgent = latestVisit.UserAgent
	}

	return &URLStats{
		ShortCode:          urlRecord.ShortCode,
		OriginalURL:        urlRecord.OriginalURL,
		ClickCount:         urlRecord.ClickCount,
		TotalVisits:        visitCount,
		LastVisitAt:        lastVisitAt,
		LastVisitUserAgent: lastVisitUserAgent,
	}, nil
}
