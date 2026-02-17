package services

import (
	"testing"
	"time"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupPreviewQueueTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	logger.Init()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = db.AutoMigrate(
		&models.User{},
		&models.File{},
		&models.PreviewJob{},
	)
	if err != nil {
		t.Fatalf("failed automigrating: %v", err)
	}

	return db
}

func TestPreviewQueueService_Enqueue(t *testing.T) {
	db := setupPreviewQueueTestDB(t)

	owner := &models.User{
		Email:        "preview-owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Preview",
		LastName:     "Owner",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	file := &models.File{
		Name:        "test.pdf",
		MimeType:    "application/pdf",
		Size:        1024,
		IsDirectory: false,
		OwnerID:     owner.ID,
		StoragePath: "test.pdf",
	}
	db.Create(file)

	t.Run("returns existing pending job without creating duplicate", func(t *testing.T) {
		existingJob := &models.PreviewJob{
			FileID:      file.ID,
			Status:      models.PreviewJobStatusPending,
			MaxAttempts: 3,
		}
		db.Create(existingJob)

		cfg := config.PreviewConfig{
			QueueBufferSize: 10,
			MaxAttempts:     3,
			RetryDelays:     []time.Duration{1 * time.Second},
		}
		previewService := NewPreviewService(db, nil, config.GotenbergConfig{})
		service := NewPreviewQueueService(db, previewService, cfg)

		job, err := service.Enqueue(file.ID, &owner.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job == nil {
			t.Fatal("expected non-nil job")
		}
		if job.ID != existingJob.ID {
			t.Error("expected same job to be returned")
		}
	})
}

func TestPreviewQueueService_GetJobByFileID(t *testing.T) {
	db := setupPreviewQueueTestDB(t)
	cfg := config.PreviewConfig{
		QueueBufferSize: 10,
		MaxAttempts:     3,
		RetryDelays:     []time.Duration{1 * time.Second},
	}
	previewService := NewPreviewService(db, nil, config.GotenbergConfig{})
	service := NewPreviewQueueService(db, previewService, cfg)

	t.Run("returns nil for non-existent file", func(t *testing.T) {
		fakeID := uuid.New()
		job, err := service.GetJobByFileID(fakeID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job != nil {
			t.Error("expected nil job")
		}
	})

	t.Run("returns job for file with existing job", func(t *testing.T) {
		owner := &models.User{
			Email:        "preview-get@test.com",
			PasswordHash: "hash",
			FirstName:    "Get",
			LastName:     "Test",
			Role:         models.UserRoleUser,
		}
		db.Create(owner)

		file := &models.File{
			Name:        "get-test.pdf",
			MimeType:    "application/pdf",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "get-test.pdf",
		}
		db.Create(file)

		previewJob := &models.PreviewJob{
			FileID:      file.ID,
			Status:      models.PreviewJobStatusPending,
			MaxAttempts: 3,
		}
		db.Create(previewJob)

		job, err := service.GetJobByFileID(file.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job == nil {
			t.Fatal("expected non-nil job")
		}
		if job.FileID != file.ID {
			t.Errorf("expected file ID %s, got %s", file.ID, job.FileID)
		}
	})
}

func TestPreviewQueueService_Retry(t *testing.T) {
	db := setupPreviewQueueTestDB(t)

	owner := &models.User{
		Email:        "preview-retry@test.com",
		PasswordHash: "hash",
		FirstName:    "Retry",
		LastName:     "Test",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	t.Run("retries failed job with remaining attempts", func(t *testing.T) {
		file := &models.File{
			Name:        "retry-test.pdf",
			MimeType:    "application/pdf",
			Size:        200,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "retry-test.pdf",
		}
		db.Create(file)

		errMsg := "conversion failed"
		failedJob := &models.PreviewJob{
			FileID:      file.ID,
			Status:      models.PreviewJobStatusFailed,
			MaxAttempts: 3,
			Attempts:    1,
			LastError:   &errMsg,
		}
		db.Create(failedJob)

		cfg := config.PreviewConfig{
			QueueBufferSize: 10,
			MaxAttempts:     3,
			RetryDelays:     []time.Duration{1 * time.Second},
		}
		previewService := NewPreviewService(db, nil, config.GotenbergConfig{})
		service := NewPreviewQueueService(db, previewService, cfg)

		job, err := service.Retry(file.ID, &owner.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job == nil {
			t.Fatal("expected non-nil job")
		}
		if job.Status != models.PreviewJobStatusPending {
			t.Errorf("expected status pending after retry, got %s", job.Status)
		}
	})
}
