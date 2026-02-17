package models

import (
	"time"

	"github.com/google/uuid"
)

type WebAuthnCredential struct {
	BaseModel
	UserID          uuid.UUID  `json:"userID" gorm:"type:uuid;index;not null"`
	CredentialID    []byte     `json:"-" gorm:"type:bytea;uniqueIndex;not null"`
	PublicKey       []byte     `json:"-" gorm:"type:bytea;not null"`
	AttestationType string     `json:"-" gorm:"type:varchar(64)"`
	AAGUID          []byte     `json:"-" gorm:"type:bytea"`
	SignCount       uint32     `json:"-" gorm:"default:0"`
	Name            string     `json:"name" gorm:"type:varchar(255);not null"`
	Transports      string     `json:"-" gorm:"type:text"`
	LastUsedAt      *time.Time `json:"lastUsedAt,omitempty"`
	BackupEligible  bool       `json:"backupEligible" gorm:"default:false"`
	BackupState     bool       `json:"backupState" gorm:"default:false"`
	User            User       `json:"-" gorm:"foreignKey:UserID"`
}
