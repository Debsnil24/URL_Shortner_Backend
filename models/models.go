package models

import (
	"time"

	uuid "github.com/google/uuid"
)

type User struct {
    ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
    Email         string    `gorm:"unique;not null"`
    PasswordHash  string    // NULL for OAuth users
    Provider      string    `gorm:"not null;default:'email'"` // 'email', 'google', 'apple'
    ProviderID    string    // OAuth provider's unique ID
    FirstName     string
    LastName      string
    AvatarURL     string
    EmailVerified bool      `gorm:"default:false"`
    IsActive      bool      `gorm:"default:true"`
    LastLogin     *time.Time
    CreatedAt     time.Time `gorm:"autoCreateTime"`
    UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

type URL struct {
	ID          uint      `gorm:"primaryKey"`
	ShortCode   string    `gorm:"size:10;unique;not null"`
	OriginalURL string    `gorm:"not null"`
	UserID      uuid.UUID `gorm:"type:uuid"`
	User        User      `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	ExpiresAt   *time.Time
	ClickCount  int
}

type URLVisit struct {
	ID        uint `gorm:"primaryKey"`
	URLID     uint
	URL       URL `gorm:"constraint:OnDelete:CASCADE;"`
	IPAddress string
	UserAgent string
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
