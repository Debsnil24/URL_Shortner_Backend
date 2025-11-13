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
		{
			ID: "20250801_auth_system_update",
			Migrate: func(tx *gorm.DB) error {
				// Add new columns to users table
				if err := tx.Exec(`
					ALTER TABLE users 
					ADD COLUMN IF NOT EXISTS provider_id VARCHAR(255),
					ADD COLUMN IF NOT EXISTS first_name VARCHAR(100),
					ADD COLUMN IF NOT EXISTS last_name VARCHAR(100),
					ADD COLUMN IF NOT EXISTS avatar_url TEXT,
					ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE,
					ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE,
					ADD COLUMN IF NOT EXISTS last_login TIMESTAMP,
					ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				`).Error; err != nil {
					return err
				}

				// Update existing users to have 'email' provider
				if err := tx.Exec(`
					UPDATE users 
					SET provider = 'email' 
					WHERE provider IS NULL OR provider = ''
				`).Error; err != nil {
					return err
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec(`
					ALTER TABLE users 
					DROP COLUMN IF EXISTS provider_id,
					DROP COLUMN IF EXISTS first_name,
					DROP COLUMN IF EXISTS last_name,
					DROP COLUMN IF EXISTS avatar_url,
					DROP COLUMN IF EXISTS email_verified,
					DROP COLUMN IF EXISTS is_active,
					DROP COLUMN IF EXISTS last_login,
					DROP COLUMN IF EXISTS updated_at
				`).Error
			},
		},
		{
			ID: "20250905_performance_optimization",
			Migrate: func(tx *gorm.DB) error {
				// Ensure proper indexing for performance
				if err := tx.Exec(`
					CREATE INDEX IF NOT EXISTS idx_users_id ON users(id);
					CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
					CREATE INDEX IF NOT EXISTS idx_users_provider_id ON users(provider_id);
				`).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec(`
					DROP INDEX IF EXISTS idx_users_id;
					DROP INDEX IF EXISTS idx_users_email;
					DROP INDEX IF EXISTS idx_users_provider_id;
				`).Error
			},
		},
		{
			ID: "20251111_url_updated_at_column",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Exec(`
					ALTER TABLE urls
					ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				`).Error; err != nil {
					return err
				}

				return tx.Exec(`
					UPDATE urls
					SET updated_at = created_at
					WHERE updated_at IS NULL
				`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec(`
					ALTER TABLE urls
					DROP COLUMN IF EXISTS updated_at
				`).Error
			},
		},
	}
}
