package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog is an append-only record of every significant action in the system.
// It does NOT use BaseModel because audit rows are never updated or soft-deleted.
type AuditLog struct {
	ID           uuid.UUID              `json:"id" gorm:"type:uuid;primaryKey"`
	UserID       *uuid.UUID             `json:"userID,omitempty" gorm:"type:uuid;index"`
	Action       string                 `json:"action" gorm:"type:varchar(50);not null;index"`
	ResourceType string                 `json:"resourceType" gorm:"type:varchar(30);not null;index"`
	ResourceID   *uuid.UUID             `json:"resourceID,omitempty" gorm:"type:uuid;index"`
	Details      map[string]interface{} `json:"details,omitempty" gorm:"type:jsonb;serializer:json"`
	IPAddress    string                 `json:"ipAddress" gorm:"type:varchar(45);not null"`
	RequestID    string                 `json:"requestID,omitempty" gorm:"type:varchar(36)"`
	CreatedAt    time.Time              `json:"createdAt" gorm:"not null;index"`
}

func (a *AuditLog) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	return nil
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditExportCursor tracks the last successful export timestamp so
// the periodic S3 export only ships new rows.
type AuditExportCursor struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	LastExportAt  time.Time `json:"lastExportAt" gorm:"not null"`
	ExportedCount int64     `json:"exportedCount" gorm:"not null;default:0"`
}

func (a *AuditExportCursor) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (AuditExportCursor) TableName() string {
	return "audit_export_cursors"
}
