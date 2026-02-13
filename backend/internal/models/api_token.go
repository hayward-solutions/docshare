package models

import (
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	BaseModel
	UserID     uuid.UUID  `json:"userID" gorm:"type:uuid;not null;index"`
	Name       string     `json:"name" gorm:"type:varchar(255);not null"`
	TokenHash  string     `json:"-" gorm:"type:text;not null;uniqueIndex"`
	Prefix     string     `json:"prefix" gorm:"type:varchar(10);not null"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty" gorm:"index"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	User       User       `json:"-" gorm:"foreignKey:UserID"`
}
