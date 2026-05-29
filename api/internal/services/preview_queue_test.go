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

func TestPreviewQueueService_RecoverStaleJobs(t *testing.T) {
	db := setupPreviewQueueTestDB(t)

	owner := &models.User{
		Email:        "preview-recover@test.com",
		PasswordHash: "hash",
		FirstName:    "Recover",
		LastName:     "Test",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	mkfile := func(name string) *models.File {
		f := &models.File{
			Name:        name,
			MimeType:    "image/png",
			Size:        1,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: name,
		}
		db.Create(f)
		return f
	}

	cfg := config.PreviewConfig{
		QueueBufferSize: 10,
		MaxAttempts:     3,
		RetryDelays:     []time.Duration{1 * time.Second},
		// StaleRecoveryInterval intentionally zero: we drive RecoverStaleJobs
		// directly and don't want a concurrent background tick racing the
		// channel-drain assertions below.
	}
	previewService := NewPreviewService(db, nil, config.GotenbergConfig{})
	// Construct without NewPreviewQueueService to keep the processQueue
	// worker out of the picture — RecoverStaleJobs is the unit under test
	// and we want to assert on the channel contents directly, not race a
	// worker (which would also nil-deref on the nil storage client).
	service := &PreviewQueueService{
		DB:             db,
		PreviewService: previewService,
		queue:          make(chan PreviewJobTask, cfg.QueueBufferSize),
		config:         cfg,
	}

	future := time.Now().UTC().Add(5 * time.Minute)
	past := time.Now().UTC().Add(-1 * time.Minute)
	earlier := time.Now().UTC().Add(-15 * time.Minute)

	// fresh pending (no NextRetryAt) — should be re-enqueued
	freshFile := mkfile("fresh.png")
	freshJob := &models.PreviewJob{FileID: freshFile.ID, Status: models.PreviewJobStatusPending, MaxAttempts: 3}
	db.Create(freshJob)

	// pending whose retry is due — should be re-enqueued
	dueFile := mkfile("due.png")
	dueJob := &models.PreviewJob{FileID: dueFile.ID, Status: models.PreviewJobStatusPending, MaxAttempts: 3, Attempts: 1, NextRetryAt: &past}
	db.Create(dueJob)

	// pending whose retry is still in the future — must NOT be re-enqueued
	scheduledFile := mkfile("scheduled.png")
	scheduledJob := &models.PreviewJob{FileID: scheduledFile.ID, Status: models.PreviewJobStatusPending, MaxAttempts: 3, Attempts: 1, NextRetryAt: &future}
	db.Create(scheduledJob)

	// stuck processing (older than 10 min) — should flip to pending and re-enqueue
	stuckFile := mkfile("stuck.png")
	stuckJob := &models.PreviewJob{FileID: stuckFile.ID, Status: models.PreviewJobStatusProcessing, MaxAttempts: 3, StartedAt: &earlier}
	db.Create(stuckJob)
	// Force updated_at backward — RecoverStaleJobs picks by status+updated_at age.
	db.Model(stuckJob).UpdateColumn("updated_at", earlier)

	service.RecoverStaleJobs()

	// Drain the channel; capture every FileID we received.
	got := map[uuid.UUID]bool{}
	timeout := time.After(200 * time.Millisecond)
collect:
	for {
		select {
		case task := <-service.queue:
			got[task.FileID] = true
		case <-timeout:
			break collect
		}
	}

	if !got[freshFile.ID] {
		t.Error("fresh pending job was not recovered")
	}
	if !got[dueFile.ID] {
		t.Error("due-retry pending job was not recovered")
	}
	if got[scheduledFile.ID] {
		t.Error("future-retry pending job should NOT have been recovered (would burn retries)")
	}
	if !got[stuckFile.ID] {
		t.Error("stuck-processing job was not recovered")
	}

	var revived models.PreviewJob
	db.First(&revived, "file_id = ?", stuckFile.ID)
	if revived.Status != models.PreviewJobStatusPending {
		t.Errorf("expected stuck job to flip to pending, got %s", revived.Status)
	}
}
