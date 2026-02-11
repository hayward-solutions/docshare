package database

import (
	"fmt"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.DBConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	if err := seedAdminUser(db); err != nil {
		return nil, err
	}

	return db, nil
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMembership{},
		&models.File{},
		&models.Share{},
	); err != nil {
		return err
	}

	constraint := `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'share_target_check'
  ) THEN
    ALTER TABLE shares
    ADD CONSTRAINT share_target_check
    CHECK (
      (shared_with_user_id IS NOT NULL AND shared_with_group_id IS NULL)
      OR
      (shared_with_user_id IS NULL AND shared_with_group_id IS NOT NULL)
    );
  END IF;
END $$;`

	return db.Exec(constraint).Error
}

func seedAdminUser(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	hash, err := utils.HashPassword("admin123")
	if err != nil {
		return err
	}

	admin := models.User{
		Email:        "admin@docshare.local",
		PasswordHash: hash,
		FirstName:    "System",
		LastName:     "Admin",
		Role:         models.UserRoleAdmin,
	}

	return db.Create(&admin).Error
}
