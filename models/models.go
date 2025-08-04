package models

import (
	"time"

	uuid "github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email        string    `gorm:"unique;not null"`
	PasswordHash string
	Provider     string
	CreatedAt    time.Time `gorm:"autoCreateTime"`
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
