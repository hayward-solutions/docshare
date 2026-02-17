package models

import "github.com/google/uuid"

type Group struct {
	BaseModel
	Name        string            `json:"name" gorm:"type:varchar(150);not null"`
	Description *string           `json:"description,omitempty" gorm:"type:text"`
	CreatedByID uuid.UUID         `json:"createdByID" gorm:"type:uuid;not null;index"`
	CreatedBy   User              `json:"createdBy" gorm:"foreignKey:CreatedByID"`
	Memberships []GroupMembership `json:"memberships,omitempty" gorm:"foreignKey:GroupID"`
	Shares      []Share           `json:"-" gorm:"foreignKey:SharedWithGroupID"`
}
