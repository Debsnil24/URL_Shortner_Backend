package controller

import (
	"errors"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
