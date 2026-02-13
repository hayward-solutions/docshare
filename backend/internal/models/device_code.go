package models

import (
	"time"

	"github.com/google/uuid"
)

type DeviceCodeStatus string

const (
	DeviceCodePending  DeviceCodeStatus = "pending"
	DeviceCodeApproved DeviceCodeStatus = "approved"
	DeviceCodeDenied   DeviceCodeStatus = "denied"
	DeviceCodeExpired  DeviceCodeStatus = "expired"
)

type DeviceCode struct {
	BaseModel
	DeviceCodeHash string           `json:"-" gorm:"type:text;not null;uniqueIndex"`
	UserCode       string           `json:"userCode" gorm:"type:varchar(16);not null;uniqueIndex"`
	ExpiresAt      time.Time        `json:"expiresAt" gorm:"not null;index"`
	Interval       int              `json:"interval" gorm:"not null;default:5"`
	Status         DeviceCodeStatus `json:"status" gorm:"type:varchar(20);not null;default:'pending';index"`
	UserID         *uuid.UUID       `json:"userID,omitempty" gorm:"type:uuid;index"`
	User           *User            `json:"-" gorm:"foreignKey:UserID"`
}
