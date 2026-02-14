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
		&models.AuditLog{},
		&models.AuditExportCursor{},
		&models.Activity{},
		&models.APIToken{},
		&models.DeviceCode{},
		&models.Transfer{},
		&models.PreviewJob{},
		&models.SSOProvider{},
		&models.LinkedAccount{},
	); err != nil {
		return err
	}

	// Drop the old constraint if it exists, then create the updated one that
	// also allows public shares (both user and group NULL when share_type is public).
	dropOld := `
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'share_target_check'
  ) THEN
    ALTER TABLE shares DROP CONSTRAINT share_target_check;
  END IF;
END $$;`

	if err := db.Exec(dropOld).Error; err != nil {
		return err
	}

	constraint := `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'share_target_check_v2'
  ) THEN
    ALTER TABLE shares
    ADD CONSTRAINT share_target_check_v2
    CHECK (
      (share_type = 'private' AND (
        (shared_with_user_id IS NOT NULL AND shared_with_group_id IS NULL)
        OR
        (shared_with_user_id IS NULL AND shared_with_group_id IS NOT NULL)
      ))
      OR
      (share_type IN ('public_anyone', 'public_logged_in') AND shared_with_user_id IS NULL AND shared_with_group_id IS NULL)
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
