package config

import (
	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func GetMigrations() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			ID: "20250801_init_schema",
			Migrate: func(tx *gorm.DB) error {
				// Ensure pgcrypto is available for gen_random_uuid()
				if err := tx.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`).Error; err != nil {
					return err
				}

				// Now run AutoMigrate on your models
				return tx.AutoMigrate(
					&models.User{},
					&models.URL{},
					&models.URLVisit{},
				)
			},
			Rollback: func(tx *gorm.DB) error {
				// Optional: drop tables in reverse dependency order
				return tx.Migrator().DropTable("url_visits", "urls", "users")
			},
		},
	}
}
