package models

import "github.com/google/uuid"

type Activity struct {
	BaseModel
	UserID       uuid.UUID  `json:"userID" gorm:"type:uuid;not null;index"`
	ActorID      uuid.UUID  `json:"actorID" gorm:"type:uuid;not null"`
	Action       string     `json:"action" gorm:"type:varchar(50);not null"`
	ResourceType string     `json:"resourceType" gorm:"type:varchar(30);not null"`
	ResourceID   *uuid.UUID `json:"resourceID,omitempty" gorm:"type:uuid"`
	ResourceName string     `json:"resourceName" gorm:"type:varchar(255);not null"`
	Message      string     `json:"message" gorm:"type:text;not null"`
	IsRead       bool       `json:"isRead" gorm:"not null;default:false;index"`

	Actor User `json:"actor,omitempty" gorm:"foreignKey:ActorID;references:ID"`
}

func (Activity) TableName() string {
	return "activities"
}
