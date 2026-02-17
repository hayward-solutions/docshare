package models

import (
	"time"

	"github.com/google/uuid"
)

type MFAConfig struct {
	BaseModel
	UserID         uuid.UUID  `json:"userID" gorm:"type:uuid;uniqueIndex;not null"`
	TOTPEnabled    bool       `json:"totpEnabled" gorm:"default:false"`
	TOTPSecret     string     `json:"-" gorm:"type:text"`
	TOTPVerifiedAt *time.Time `json:"totpVerifiedAt,omitempty"`
	RecoveryCodes  string     `json:"-" gorm:"type:text"`
	RecoveryCount  int        `json:"recoveryCodesRemaining" gorm:"default:0"`
	User           User       `json:"-" gorm:"foreignKey:UserID"`
}
