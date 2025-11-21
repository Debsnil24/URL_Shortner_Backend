package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	Status             string
	ClickCount         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExpiresAt          *time.Time
	TotalVisits        int64
	UniqueVisitors     int64
	LastVisitAt        *time.Time
	LastVisitUserAgent *string
	QRCodeAvailable    bool
	QRCodeSize         int
	QRCodeFormat       string
	QRCodeGeneratedAt  *time.Time
}

// URLStats represents statistics for a URL
type URLStats struct {
	ShortCode          string
	OriginalURL        string
	Status             string
	ClickCount         int
	TotalVisits        int64
	UniqueVisitors     int64
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

// CalculateExpiration calculates the expiration date from preset or custom expiration
func (c *URLController) CalculateExpiration(preset string, customExp *models.CustomExpiration, createdAt time.Time) (*time.Time, error) {
	var expiresAt time.Time

	// Priority: custom_expiration > expiration_preset > default (5 years)
	if customExp != nil {
		// Parse custom expiration values
		years, err := strconv.Atoi(customExp.Years)
		if err != nil || years < 0 || years > 4 {
			return nil, errors.New("years must be between 0 and 4")
		}

		months, err := strconv.Atoi(customExp.Months)
		if err != nil || months < 0 || months > 11 {
			return nil, errors.New("months must be between 0 and 11")
		}

		days, err := strconv.Atoi(customExp.Days)
		if err != nil || days < 0 || days > 30 {
			return nil, errors.New("days must be between 0 and 30")
		}

		hours, err := strconv.Atoi(customExp.Hours)
		if err != nil || hours < 0 || hours > 23 {
			return nil, errors.New("hours must be between 0 and 23")
		}

		minutes, err := strconv.Atoi(customExp.Minutes)
		if err != nil || minutes < 0 || minutes > 59 {
			return nil, errors.New("minutes must be between 0 and 59")
		}

		// Calculate total days: years*365 + months*30 + days + hours/24 + minutes/(24*60)
		totalDays := float64(years*365 + months*30 + days)
		totalDays += float64(hours) / 24.0
		totalDays += float64(minutes) / (24.0 * 60.0)

		// Validate: must be less than 5 years (1825 days)
		if totalDays >= 1825 {
			return nil, errors.New("custom expiration cannot exceed 5 years (1825 days)")
		}

		// Add to current time
		expiresAt = createdAt.AddDate(years, months, days)
		expiresAt = expiresAt.Add(time.Duration(hours) * time.Hour)
		expiresAt = expiresAt.Add(time.Duration(minutes) * time.Minute)

		// Validate it's in the future
		if !expiresAt.After(createdAt) {
			return nil, errors.New("expiration date must be in the future")
		}

		return &expiresAt, nil
	}

	// Handle preset expiration
	if preset != "" && preset != "default" {
		switch preset {
		case "1hour":
			expiresAt = createdAt.Add(1 * time.Hour)
		case "12hours":
			expiresAt = createdAt.Add(12 * time.Hour)
		case "1day":
			expiresAt = createdAt.AddDate(0, 0, 1)
		case "7days":
			expiresAt = createdAt.AddDate(0, 0, 7)
		case "1month":
			expiresAt = createdAt.AddDate(0, 1, 0) // 30 days
		case "6months":
			expiresAt = createdAt.AddDate(0, 6, 0) // ~180 days
		case "1year":
			expiresAt = createdAt.AddDate(1, 0, 0) // 365 days
		default:
			return nil, fmt.Errorf("invalid expiration preset: %s. Valid values: 1hour, 12hours, 1day, 7days, 1month, 6months, 1year, default", preset)
		}

		// Validate it's in the future (should always be, but double-check)
		if !expiresAt.After(createdAt) {
			return nil, errors.New("expiration date must be in the future")
		}

		return &expiresAt, nil
	}

	// Default: 5 years from now
	expiresAt = createdAt.AddDate(5, 0, 0)
	return &expiresAt, nil
}

func (c *URLController) GenerateShortCode(originalURL string, userID uuid.UUID, preset string, customExp *models.CustomExpiration) (*models.URL, error) {
	const maxAttempts = 10 // Maximum attempts to generate a unique short code

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Generate short code using util function
		code := util.GenerateShortCode()

		// Check if this code already exists in database
		var existingURL models.URL
		if err := c.DB.Where("short_code = ?", code).First(&existingURL).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				// Database error, abort and surface error
				return nil, err
			}

			// Code doesn't exist - create it
			createdAt := time.Now()

			// Calculate expiration date
			expiresAt, err := c.CalculateExpiration(preset, customExp, createdAt)
			if err != nil {
				return nil, err
			}

			urlRecord := models.URL{
				ShortCode:   code,
				OriginalURL: originalURL,
				ClickCount:  0,
				UserID:      userID,
				Status:      "active",
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
				ExpiresAt:   expiresAt,
			}

			if err := c.DB.Create(&urlRecord).Error; err != nil {
				return nil, err
			}

			return &urlRecord, nil
		}

		// Code exists, try again
	}

	// All attempts exhausted - return error to prevent infinite loop
	return nil, errors.New("failed to generate unique short code after maximum attempts")
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
// Excludes QR code image binary data to optimize performance
func (c *URLController) ListURLsByUser(userID uuid.UUID) ([]URLSummary, error) {
	var urls []models.URL
	// Select specific columns to exclude qr_code_image (BYTEA) for performance
	// This prevents loading large binary data when listing URLs
	if err := c.DB.Select("id, short_code, original_url, user_id, status, created_at, updated_at, expires_at, click_count, qr_code_size, qr_code_format, qr_code_generated_at").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&urls).Error; err != nil {
		return nil, err
	}

	summaries := make([]URLSummary, 0, len(urls))
	for _, urlRecord := range urls {
		visitCount, err := c.GetVisitCount(urlRecord.ID)
		if err != nil {
			visitCount = 0 // Continue even if count fails
		}

		uniqueVisitors, err := c.GetUniqueVisitorCount(urlRecord.ID)
		if err != nil {
			uniqueVisitors = 0 // Continue even if count fails
		}

		latestVisit, err := c.GetLatestVisit(urlRecord.ID)
		var lastVisitAt *time.Time
		var lastVisitUserAgent *string
		if err == nil && latestVisit != nil {
			lastVisitAt = &latestVisit.CreatedAt
			ua := latestVisit.UserAgent
			lastVisitUserAgent = &ua
		}

		// Use TotalVisits as the source of truth for click count to ensure consistency
		// TotalVisits is the actual count from url_visits table, which is more reliable
		// If visitCount doesn't match ClickCount, prefer visitCount (the actual data)
		displayClickCount := int(visitCount)
		if displayClickCount == 0 && urlRecord.ClickCount > 0 {
			// Fallback to stored click_count if visitCount is 0 but click_count exists
			// This handles edge cases where visits table might be empty
			displayClickCount = urlRecord.ClickCount
		}

		// Check if QR code is available (without loading binary data)
		qrCodeAvailable := urlRecord.QRCodeGeneratedAt != nil && urlRecord.QRCodeSize > 0

		summaries = append(summaries, URLSummary{
			ShortCode:          urlRecord.ShortCode,
			OriginalURL:        urlRecord.OriginalURL,
			Status:             urlRecord.Status,
			ClickCount:         displayClickCount, // Use TotalVisits as source of truth
			CreatedAt:          urlRecord.CreatedAt,
			UpdatedAt:          urlRecord.UpdatedAt,
			ExpiresAt:          urlRecord.ExpiresAt,
			TotalVisits:        visitCount,
			UniqueVisitors:     uniqueVisitors,
			LastVisitAt:        lastVisitAt,
			LastVisitUserAgent: lastVisitUserAgent,
			QRCodeAvailable:    qrCodeAvailable,
			QRCodeSize:         urlRecord.QRCodeSize,
			QRCodeFormat:       urlRecord.QRCodeFormat,
			QRCodeGeneratedAt:  urlRecord.QRCodeGeneratedAt,
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

// UpdateURL updates a URL's original URL and/or expiration date
func (c *URLController) UpdateURL(code string, userID uuid.UUID, req *models.UpdateURLRequest) (*models.URL, error) {
	// Get existing URL
	var urlRecord models.URL
	if err := c.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("URL not found")
		}
		return nil, err
	}

	// Check ownership
	if urlRecord.UserID != userID {
		return nil, errors.New("permission denied")
	}

	// Check if at least one field is provided
	hasURLUpdate := strings.TrimSpace(req.URL) != ""
	hasExpirationUpdate := req.ExpirationPreset != "" || req.CustomExpiration != nil
	hasBothExpiration := req.ExpirationPreset != "" && req.CustomExpiration != nil

	if !hasURLUpdate && !hasExpirationUpdate {
		return nil, errors.New("at least one field (url, expiration_preset, or custom_expiration) must be provided")
	}

	if hasBothExpiration {
		return nil, errors.New("cannot provide both expiration_preset and custom_expiration. Use only one")
	}

	now := time.Now()

	// Check if link is expired (expires_at has passed)
	isExpired := urlRecord.ExpiresAt != nil && (urlRecord.ExpiresAt.Before(now) || urlRecord.ExpiresAt.Equal(now))

	// Special case: Expired link handling
	if isExpired {
		// If expired and only URL is being updated (no expiration update), return 410
		if hasURLUpdate && !hasExpirationUpdate {
			return nil, errors.New("cannot update URL of expired link. Update expiration to reactivate it")
		}
		// If expired and expiration is being updated, allow it (reactivation)
	}

	// Validate and update URL
	if hasURLUpdate {
		newURL := strings.TrimSpace(req.URL)
		if newURL == "" {
			return nil, errors.New("url cannot be empty")
		}
		// Ensure URL starts with http:// or https://
		if !strings.HasPrefix(newURL, "http://") && !strings.HasPrefix(newURL, "https://") {
			newURL = "https://" + newURL
		}
		// Basic URL validation
		if len(newURL) < 10 { // Very basic validation - should have at least "https://x.co"
			return nil, errors.New("invalid URL format")
		}
		urlRecord.OriginalURL = newURL
	}

	// Validate and update expiration
	if hasExpirationUpdate {
		// Calculate new expiration date using the same logic as creation
		newExpiresAt, err := c.CalculateExpiration(req.ExpirationPreset, req.CustomExpiration, now)
		if err != nil {
			return nil, err
		}
		urlRecord.ExpiresAt = newExpiresAt
	}

	// Update timestamp
	urlRecord.UpdatedAt = now

	// Save changes
	if err := c.DB.Save(&urlRecord).Error; err != nil {
		return nil, err
	}

	return &urlRecord, nil
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

// RecordVisitAndIncrement atomically records a visit and increments the click count
// This ensures both operations succeed or fail together, preventing data inconsistency
func (c *URLController) RecordVisitAndIncrement(urlID uint, ipAddress, userAgent string) error {
	// Use a transaction to ensure atomicity
	return c.DB.Transaction(func(tx *gorm.DB) error {
		// First, create the visit record
		visit := models.URLVisit{
			URLID:     urlID,
			IPAddress: ipAddress,
			UserAgent: userAgent,
		}
		if err := tx.Create(&visit).Error; err != nil {
			return err
		}

		// Then, increment the click count
		if err := tx.Model(&models.URL{}).Where("id = ?", urlID).UpdateColumn("click_count", gorm.Expr("click_count + ?", 1)).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetVisitCount returns the total number of visits for a URL
func (c *URLController) GetVisitCount(urlID uint) (int64, error) {
	var count int64
	if err := c.DB.Model(&models.URLVisit{}).Where("url_id = ?", urlID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetUniqueVisitorCount returns the number of unique visitors (distinct IP addresses) for a URL
func (c *URLController) GetUniqueVisitorCount(urlID uint) (int64, error) {
	var count int64
	// Count distinct IP addresses for this URL using PostgreSQL-compatible query
	// Using Select with Distinct and Count for better compatibility
	if err := c.DB.Model(&models.URLVisit{}).
		Where("url_id = ?", urlID).
		Select("COUNT(DISTINCT ip_address)").
		Scan(&count).Error; err != nil {
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

	uniqueVisitors, err := c.GetUniqueVisitorCount(urlRecord.ID)
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

	// Use TotalVisits as the source of truth for click count to ensure consistency
	// TotalVisits is the actual count from url_visits table, which is more reliable
	displayClickCount := int(visitCount)
	if displayClickCount == 0 && urlRecord.ClickCount > 0 {
		// Fallback to stored click_count if visitCount is 0 but click_count exists
		// This handles edge cases where visits table might be empty
		displayClickCount = urlRecord.ClickCount
	}

	return &URLStats{
		ShortCode:          urlRecord.ShortCode,
		OriginalURL:        urlRecord.OriginalURL,
		Status:             urlRecord.Status,
		ClickCount:         displayClickCount, // Use TotalVisits as source of truth
		TotalVisits:        visitCount,
		UniqueVisitors:     uniqueVisitors,
		LastVisitAt:        lastVisitAt,
		LastVisitUserAgent: lastVisitUserAgent,
	}, nil
}

// UpdateURLStatus updates only the status field of a URL
func (c *URLController) UpdateURLStatus(code string, userID uuid.UUID, status string) (*models.URL, error) {
	// Validate status value
	if status != "active" && status != "paused" {
		return nil, errors.New("status must be either 'active' or 'paused'")
	}

	// Get existing URL
	var urlRecord models.URL
	if err := c.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("URL not found")
		}
		return nil, err
	}

	// Check ownership
	if urlRecord.UserID != userID {
		return nil, errors.New("permission denied")
	}

	// Update status and timestamp
	urlRecord.Status = status
	urlRecord.UpdatedAt = time.Now()

	// Save changes
	if err := c.DB.Save(&urlRecord).Error; err != nil {
		return nil, err
	}

	return &urlRecord, nil
}

// GenerateQRCode generates a QR code for a URL and stores it in the database
// If QR code already exists, it will be regenerated with the new size
func (c *URLController) GenerateQRCode(code string, userID uuid.UUID, size int) (*models.URL, error) {
	// Get existing URL
	var urlRecord models.URL
	if err := c.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("URL not found")
		}
		return nil, err
	}

	// Check ownership
	if urlRecord.UserID != userID {
		return nil, errors.New("permission denied")
	}

	// Build the full short URL for QR code generation
	shortURL := fmt.Sprintf("https://www.sniply.co.in/%s", code)

	// Generate QR code using utility function
	qrCodeBytes, err := util.GenerateQRCode(shortURL, size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Update URL record with QR code data
	now := time.Now()
	urlRecord.QRCodeImage = qrCodeBytes
	urlRecord.QRCodeSize = size
	urlRecord.QRCodeFormat = "png"
	urlRecord.QRCodeGeneratedAt = &now
	urlRecord.UpdatedAt = now

	// Save changes
	if err := c.DB.Save(&urlRecord).Error; err != nil {
		return nil, err
	}

	return &urlRecord, nil
}

// GetQRCode retrieves the QR code image for a URL
// If QR code doesn't exist, it will be generated automatically with default size
func (c *URLController) GetQRCode(code string, userID uuid.UUID) ([]byte, int, string, error) {
	// Get existing URL
	var urlRecord models.URL
	if err := c.DB.Where("short_code = ?", code).First(&urlRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, "", errors.New("URL not found")
		}
		return nil, 0, "", err
	}

	// Check ownership
	if urlRecord.UserID != userID {
		return nil, 0, "", errors.New("permission denied")
	}

	// If QR code doesn't exist, generate it with default size
	if len(urlRecord.QRCodeImage) == 0 {
		// Generate QR code with default size
		generatedURL, err := c.GenerateQRCode(code, userID, util.DefaultQRCodeSize)
		if err != nil {
			return nil, 0, "", err
		}
		return generatedURL.QRCodeImage, generatedURL.QRCodeSize, generatedURL.QRCodeFormat, nil
	}

	// Return existing QR code
	return urlRecord.QRCodeImage, urlRecord.QRCodeSize, urlRecord.QRCodeFormat, nil
}
