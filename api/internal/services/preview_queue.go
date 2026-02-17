package services

import (
	"context"
	"fmt"
	"time"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PreviewJobTask struct {
	FileID        uuid.UUID
	RequestedByID *uuid.UUID
}

type PreviewQueueService struct {
	DB             *gorm.DB
	PreviewService *PreviewService
	queue          chan PreviewJobTask
	config         config.PreviewConfig
}

func NewPreviewQueueService(db *gorm.DB, previewService *PreviewService, cfg config.PreviewConfig) *PreviewQueueService {
	s := &PreviewQueueService{
		DB:             db,
		PreviewService: previewService,
		queue:          make(chan PreviewJobTask, cfg.QueueBufferSize),
		config:         cfg,
	}
	go s.processQueue()
	return s
}

func (s *PreviewQueueService) Enqueue(fileID uuid.UUID, requestedByID *uuid.UUID) (*models.PreviewJob, error) {
	var existingJob models.PreviewJob
	err := s.DB.Where("file_id = ? AND status IN ?", fileID, []string{"pending", "processing"}).
		Order("created_at DESC").
		First(&existingJob).Error

	if err == nil {
		return &existingJob, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing job: %w", err)
	}

	job := models.PreviewJob{
		FileID:        fileID,
		RequestedByID: requestedByID,
		Status:        models.PreviewJobStatusPending,
		MaxAttempts:   s.config.MaxAttempts,
	}

	if err := s.DB.Create(&job).Error; err != nil {
		return nil, fmt.Errorf("failed to create preview job: %w", err)
	}

	select {
	case s.queue <- PreviewJobTask{FileID: fileID, RequestedByID: requestedByID}:
		logger.Info("preview_job_enqueued", map[string]interface{}{
			"job_id":  job.ID.String(),
			"file_id": fileID.String(),
		})
	default:
		logger.Warn("preview_queue_full", map[string]interface{}{
			"job_id":  job.ID.String(),
			"file_id": fileID.String(),
		})
	}

	return &job, nil
}

func (s *PreviewQueueService) GetJobByFileID(fileID uuid.UUID) (*models.PreviewJob, error) {
	var job models.PreviewJob
	err := s.DB.Where("file_id = ?", fileID).
		Order("created_at DESC").
		First(&job).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *PreviewQueueService) Retry(fileID uuid.UUID, requestedByID *uuid.UUID) (*models.PreviewJob, error) {
	existing, err := s.GetJobByFileID(fileID)
	if err != nil {
		return nil, err
	}

	if existing != nil && existing.Status == models.PreviewJobStatusFailed && existing.Attempts < existing.MaxAttempts {
		existing.Status = models.PreviewJobStatusPending
		existing.LastError = nil
		existing.NextRetryAt = nil

		if err := s.DB.Save(existing).Error; err != nil {
			return nil, fmt.Errorf("failed to update job: %w", err)
		}

		select {
		case s.queue <- PreviewJobTask{FileID: fileID, RequestedByID: requestedByID}:
		default:
			logger.Warn("preview_queue_full_on_retry", map[string]interface{}{
				"job_id": existing.ID.String(),
			})
		}

		return existing, nil
	}

	return s.Enqueue(fileID, requestedByID)
}

func (s *PreviewQueueService) processQueue() {
	for task := range s.queue {
		s.processJob(task)
	}
}

func (s *PreviewQueueService) processJob(task PreviewJobTask) {
	ctx := context.Background()

	var job models.PreviewJob
	err := s.DB.Where("file_id = ? AND status = ?", task.FileID, models.PreviewJobStatusPending).
		Order("created_at DESC").
		First(&job).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return
		}
		logger.Error("preview_job_load_failed", err, map[string]interface{}{
			"file_id": task.FileID.String(),
		})
		return
	}

	now := time.Now().UTC()
	job.Status = models.PreviewJobStatusProcessing
	job.StartedAt = &now

	if err := s.DB.Save(&job).Error; err != nil {
		logger.Error("preview_job_update_failed", err, map[string]interface{}{
			"job_id": job.ID.String(),
		})
		return
	}

	var file models.File
	if err := s.DB.First(&file, "id = ?", task.FileID).Error; err != nil {
		s.markJobFailed(&job, fmt.Errorf("file not found: %w", err))
		return
	}

	previewURL, err := s.PreviewService.ConvertToPreview(ctx, &file)
	if err != nil {
		s.markJobFailed(&job, err)
		return
	}

	_ = previewURL

	completedAt := time.Now().UTC()
	job.Status = models.PreviewJobStatusCompleted
	job.CompletedAt = &completedAt

	if err := s.DB.Save(&job).Error; err != nil {
		logger.Error("preview_job_complete_failed", err, map[string]interface{}{
			"job_id": job.ID.String(),
		})
		return
	}

	logger.Info("preview_job_completed", map[string]interface{}{
		"job_id":  job.ID.String(),
		"file_id": task.FileID.String(),
	})
}

func (s *PreviewQueueService) markJobFailed(job *models.PreviewJob, jobErr error) {
	job.Attempts++
	errStr := jobErr.Error()
	job.LastError = &errStr

	if job.Attempts >= job.MaxAttempts {
		job.Status = models.PreviewJobStatusFailed
		logger.Error("preview_job_final_failure", jobErr, map[string]interface{}{
			"job_id":   job.ID.String(),
			"file_id":  job.FileID.String(),
			"attempts": job.Attempts,
		})
	} else {
		job.Status = models.PreviewJobStatusPending

		delayIndex := job.Attempts - 1
		if delayIndex >= len(s.config.RetryDelays) {
			delayIndex = len(s.config.RetryDelays) - 1
		}
		nextRetry := time.Now().UTC().Add(s.config.RetryDelays[delayIndex])
		job.NextRetryAt = &nextRetry

		logger.Warn("preview_job_retry_scheduled", map[string]interface{}{
			"job_id":       job.ID.String(),
			"file_id":      job.FileID.String(),
			"attempts":     job.Attempts,
			"max_attempts": job.MaxAttempts,
			"next_retry":   nextRetry.String(),
		})
	}

	if err := s.DB.Save(job).Error; err != nil {
		logger.Error("preview_job_failed_update_failed", err, map[string]interface{}{
			"job_id": job.ID.String(),
		})
	}
}

func (s *PreviewQueueService) RecoverStaleJobs() {
	var staleJobs []models.PreviewJob

	s.DB.Where("status = ? AND updated_at < ?", models.PreviewJobStatusProcessing, time.Now().UTC().Add(-10*time.Minute)).
		Find(&staleJobs)

	for _, job := range staleJobs {
		job.Status = models.PreviewJobStatusPending
		job.NextRetryAt = nil

		if err := s.DB.Save(&job).Error; err != nil {
			logger.Error("preview_job_stale_recovery_failed", err, map[string]interface{}{
				"job_id": job.ID.String(),
			})
			continue
		}

		select {
		case s.queue <- PreviewJobTask{FileID: job.FileID, RequestedByID: job.RequestedByID}:
		default:
			logger.Warn("preview_queue_full_on_recovery", map[string]interface{}{
				"job_id": job.ID.String(),
			})
		}

		logger.Info("preview_job_stale_recovered", map[string]interface{}{
			"job_id":  job.ID.String(),
			"file_id": job.FileID.String(),
		})
	}

	var pendingJobs []models.PreviewJob
	s.DB.Where("status = ?", models.PreviewJobStatusPending).
		Or("status = ? AND next_retry_at <= ?", models.PreviewJobStatusFailed, time.Now().UTC()).
		Find(&pendingJobs)

	for _, job := range pendingJobs {
		select {
		case s.queue <- PreviewJobTask{FileID: job.FileID, RequestedByID: job.RequestedByID}:
		default:
			logger.Warn("preview_queue_full_on_recovery", map[string]interface{}{
				"job_id": job.ID.String(),
			})
		}
	}
}
