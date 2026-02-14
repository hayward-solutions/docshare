package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PreviewJobStatus represents the state of a preview generation job.
type PreviewJobStatus string

const (
	PreviewJobStatusPending    PreviewJobStatus = "pending"
	PreviewJobStatusProcessing PreviewJobStatus = "processing"
	PreviewJobStatusCompleted  PreviewJobStatus = "completed"
	PreviewJobStatusFailed     PreviewJobStatus = "failed"
)

// PreviewJob tracks the asynchronous generation of document previews.
type PreviewJob struct {
	ID            uuid.UUID        `json:"id" gorm:"type:uuid;primaryKey"`
	FileID        uuid.UUID        `json:"fileID" gorm:"type:uuid;not null;index"`
	Status        PreviewJobStatus `json:"status" gorm:"type:varchar(20);not null;default:pending;index"`
	Attempts      int              `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts   int              `json:"maxAttempts" gorm:"not null;default:3"`
	LastError     *string          `json:"lastError,omitempty" gorm:"type:text"`
	NextRetryAt   *time.Time       `json:"nextRetryAt,omitempty" gorm:"index"`
	StartedAt     *time.Time       `json:"startedAt,omitempty"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty"`
	RequestedByID *uuid.UUID       `json:"requestedByID,omitempty" gorm:"type:uuid;index"`
	CreatedAt     time.Time        `json:"createdAt" gorm:"not null"`
	UpdatedAt     time.Time        `json:"updatedAt" gorm:"not null"`
	DeletedAt     gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (p *PreviewJob) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (p *PreviewJob) BeforeUpdate(_ *gorm.DB) error {
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (PreviewJob) TableName() string {
	return "preview_jobs"
}
