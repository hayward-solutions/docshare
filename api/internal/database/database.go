package database

import (
	"fmt"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/utils"
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

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
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
		&models.MFAConfig{},
		&models.WebAuthnCredential{},
		&models.MFAChallenge{},
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

	if err := db.Exec(constraint).Error; err != nil {
		return err
	}

	// Reconcile any pre-existing duplicate storage_path rows BEFORE adding
	// the unique index — otherwise CREATE UNIQUE INDEX fails and the API
	// won't start on environments that ran finalize concurrently before
	// this guard existed.
	//
	// For each non-empty storage_path with more than one live row, pick the
	// earliest row as the survivor and remap dependent records (shares,
	// preview jobs, and file-typed activity/audit rows) to the survivor's
	// id. Then soft-delete the duplicates. Without the remap, dependent
	// rows would point at hidden ids and default-scoped lookups would 404.
	//
	// The CTE returns (loser_id, winner_id) pairs we can join against.
	remapDuplicateRefs := `
WITH ranked AS (
  SELECT id,
         storage_path,
         ROW_NUMBER() OVER (
           PARTITION BY storage_path
           ORDER BY created_at ASC, id ASC
         ) AS rn
  FROM files
  WHERE storage_path <> ''
    AND deleted_at IS NULL
),
winners AS (SELECT id, storage_path FROM ranked WHERE rn = 1),
losers AS (SELECT id, storage_path FROM ranked WHERE rn > 1),
pairs AS (
  SELECT l.id AS loser_id, w.id AS winner_id
  FROM losers l
  JOIN winners w ON w.storage_path = l.storage_path
),
update_shares AS (
  UPDATE shares s SET file_id = p.winner_id
  FROM pairs p WHERE s.file_id = p.loser_id
  RETURNING 1
),
update_preview_jobs AS (
  UPDATE preview_jobs pj SET file_id = p.winner_id
  FROM pairs p WHERE pj.file_id = p.loser_id
  RETURNING 1
),
update_activities AS (
  UPDATE activities a SET resource_id = p.winner_id
  FROM pairs p WHERE a.resource_id = p.loser_id AND a.resource_type = 'file'
  RETURNING 1
),
update_audit_logs AS (
  UPDATE audit_logs al SET resource_id = p.winner_id
  FROM pairs p WHERE al.resource_id = p.loser_id AND al.resource_type = 'file'
  RETURNING 1
)
SELECT 1;`

	if err := db.Exec(remapDuplicateRefs).Error; err != nil {
		return err
	}

	dedupeFiles := `
UPDATE files
SET deleted_at = NOW()
WHERE id IN (
  SELECT id FROM (
    SELECT id,
           ROW_NUMBER() OVER (
             PARTITION BY storage_path
             ORDER BY created_at ASC, id ASC
           ) AS rn
    FROM files
    WHERE storage_path <> ''
      AND deleted_at IS NULL
  ) ranked
  WHERE rn > 1
);`

	if err := db.Exec(dedupeFiles).Error; err != nil {
		return err
	}

	// Defends FinalizeUpload's replay check against concurrent racers: with
	// only an application-level Count→Create gap, two parallel finalize calls
	// could both observe no row and both insert. The partial unique index
	// excludes directories (which legitimately share storage_path = '')
	// AND soft-deleted rows (so finalize's default-scoped pre-check stays
	// consistent with the index — a deleted row's storage_path is freed).
	storagePathUnique := `
CREATE UNIQUE INDEX IF NOT EXISTS files_storage_path_unique
ON files (storage_path)
WHERE storage_path <> '' AND deleted_at IS NULL;`

	return db.Exec(storagePathUnique).Error
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
