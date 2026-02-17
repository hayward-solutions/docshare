package models

import (
	"time"

	"github.com/google/uuid"
)

type SharePermission string

const (
	SharePermissionView     SharePermission = "view"
	SharePermissionDownload SharePermission = "download"
	SharePermissionEdit     SharePermission = "edit"
)

type ShareType string

const (
	ShareTypePrivate        ShareType = "private"
	ShareTypePublicAnyone   ShareType = "public_anyone"
	ShareTypePublicLoggedIn ShareType = "public_logged_in"
)

type Share struct {
	BaseModel
	FileID            uuid.UUID       `json:"fileID" gorm:"type:uuid;not null;index"`
	SharedByID        uuid.UUID       `json:"sharedByID" gorm:"type:uuid;not null;index"`
	SharedWithUserID  *uuid.UUID      `json:"sharedWithUserID,omitempty" gorm:"type:uuid;index"`
	SharedWithGroupID *uuid.UUID      `json:"sharedWithGroupID,omitempty" gorm:"type:uuid;index"`
	ShareType         ShareType       `json:"shareType" gorm:"type:varchar(20);not null;default:'private';index"`
	Permission        SharePermission `json:"permission" gorm:"type:varchar(20);not null;default:'view'"`
	ExpiresAt         *time.Time      `json:"expiresAt,omitempty"`
	File              File            `json:"file,omitempty" gorm:"foreignKey:FileID;references:ID"`
	SharedBy          User            `json:"sharedBy,omitempty" gorm:"foreignKey:SharedByID;references:ID"`
	SharedWithUser    *User           `json:"sharedWithUser,omitempty" gorm:"foreignKey:SharedWithUserID;references:ID"`
	SharedWithGroup   *Group          `json:"sharedWithGroup,omitempty" gorm:"foreignKey:SharedWithGroupID;references:ID"`
}

func (Share) TableName() string {
	return "shares"
}

// IsPublic returns true if this share grants public access.
func (s *Share) IsPublic() bool {
	return s.ShareType == ShareTypePublicAnyone || s.ShareType == ShareTypePublicLoggedIn
}
