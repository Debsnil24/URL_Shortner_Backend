package controller

import (
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

func (c *URLController) GenerateShortCode(originalURL string) (*models.URL, error) {
	for {
		// Generate short code using util function
		code := util.GenerateShortCode()

		// Check if this code already exists in database
		var existingURL models.URL
		if err := c.DB.Where("short_code = ?", code).First(&existingURL).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Code is unique, create the URL record
				createdAt := time.Now()
				expiresAt := createdAt.AddDate(5, 0, 0)
				
				// Create user first
				user := models.User{
					ID:           uuid.New(),
					Email:        "test@test.com",
					PasswordHash: "test",
					Provider:     "local",
					CreatedAt:    createdAt,
				}
				
				if err := c.DB.Create(&user).Error; err != nil {
					return nil, err
				}

				// Create URL with the user reference
				urlRecord := models.URL{
					ShortCode:   code,
					OriginalURL: originalURL,
					ClickCount:  0,
					UserID:      user.ID,
					CreatedAt:   createdAt,
					ExpiresAt:   &expiresAt,
				}

				if err := c.DB.Create(&urlRecord).Error; err != nil {
					return nil, err
				}

				return &urlRecord, nil
			}
			// Database error, try again
			continue
		}

		// Code exists, generate a new one
		continue
	}
}

func (c *URLController) DeleteURL(code string) error {
	if err := c.DB.Where("short_code = ?", code).Delete(&models.URL{}).Error; err != nil {
		return err
	}
	return nil
}
