package models

import (
	"time"

	"github.com/google/uuid"
)

type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusActive    TransferStatus = "active"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusCancelled TransferStatus = "cancelled"
	TransferStatusExpired   TransferStatus = "expired"
)

type Transfer struct {
	BaseModel
	Code        string         `json:"code" gorm:"size:10;uniqueIndex"`
	SenderID    uuid.UUID      `json:"senderID" gorm:"type:uuid;not null;index"`
	Sender      User           `json:"sender,omitempty" gorm:"foreignKey:SenderID"`
	RecipientID *uuid.UUID     `json:"recipientID,omitempty" gorm:"type:uuid;index"`
	Recipient   *User          `json:"recipient,omitempty" gorm:"foreignKey:RecipientID"`
	FileName    string         `json:"fileName" gorm:"size:255;not null"`
	FileSize    int64          `json:"fileSize"`
	Status      TransferStatus `json:"status" gorm:"size:20;not null;default:'pending'"`
	Timeout     int            `json:"timeout"`
	ExpiresAt   time.Time      `json:"expiresAt"`
}

func (Transfer) TableName() string {
	return "transfers"
}
