package models

import (
	"time"

	uuid "github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email         string     `json:"email" gorm:"unique;not null"`
	PasswordHash  string     `json:"-"`                                        // NULL for OAuth users, exclude from JSON
	Provider      string     `json:"provider" gorm:"not null;default:'email'"` // 'email', 'google', 'apple'
	ProviderID    string     `json:"provider_id"`                              // OAuth provider's unique ID
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	AvatarURL     string     `json:"avatar_url"`
	EmailVerified bool       `json:"email_verified" gorm:"default:false"`
	IsActive      bool       `json:"is_active" gorm:"default:true"`
	LastLogin     *time.Time `json:"last_login"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

type URL struct {
	ID                uint      `gorm:"primaryKey"`
	ShortCode         string    `gorm:"size:10;unique;not null"`
	OriginalURL       string    `gorm:"not null"`
	UserID            uuid.UUID `gorm:"type:uuid"`
	User              User      `gorm:"constraint:OnDelete:CASCADE;"`
	Status            string    `json:"status" gorm:"type:varchar(10);default:'active';not null"`
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
	ExpiresAt         *time.Time
	ClickCount        int
	QRCodeImage       []byte     `json:"-" gorm:"type:bytea"`                                   // Exclude from JSON, store as BYTEA
	QRCodeSize        int        `json:"qr_code_size,omitempty" gorm:"default:256"`             // Size in pixels
	QRCodeFormat      string     `json:"qr_code_format,omitempty" gorm:"size:10;default:'png'"` // Format: png, svg
	QRCodeGeneratedAt *time.Time `json:"qr_code_generated_at,omitempty"`                        // Generation timestamp
}

type URLVisit struct {
	ID        uint `gorm:"primaryKey"`
	URLID     uint
	URL       URL `gorm:"constraint:OnDelete:CASCADE;"`
	IPAddress string
	UserAgent string
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
