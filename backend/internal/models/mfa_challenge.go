package models

import (
	"time"

	"github.com/google/uuid"
)

type MFAChallengeType string

const (
	MFAChallengeRegistration   MFAChallengeType = "registration"
	MFAChallengeAuthentication MFAChallengeType = "authentication"
)

type MFAChallenge struct {
	BaseModel
	UserID      *uuid.UUID       `json:"-" gorm:"type:uuid;index"`
	Challenge   []byte           `json:"-" gorm:"type:bytea;not null"`
	Type        MFAChallengeType `json:"-" gorm:"type:varchar(20);not null"`
	SessionData string           `json:"-" gorm:"type:text;not null"`
	ExpiresAt   time.Time        `json:"-" gorm:"not null;index"`
}
