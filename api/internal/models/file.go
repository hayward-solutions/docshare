package models

import "github.com/google/uuid"

type File struct {
	BaseModel
	Name          string     `json:"name" gorm:"type:varchar(255);not null"`
	MimeType      string     `json:"mimeType" gorm:"type:varchar(255);not null"`
	Size          int64      `json:"size" gorm:"not null;default:0"`
	IsDirectory   bool       `json:"isDirectory" gorm:"not null;default:false;index"`
	ParentID      *uuid.UUID `json:"parentID,omitempty" gorm:"type:uuid;index"`
	OwnerID       uuid.UUID  `json:"ownerID" gorm:"type:uuid;not null;index"`
	StoragePath   string     `json:"storagePath" gorm:"type:text;not null"`
	ThumbnailPath *string    `json:"thumbnailPath,omitempty" gorm:"type:text"`

	Parent     *File   `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children   []File  `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Owner      User    `json:"owner,omitempty" gorm:"foreignKey:OwnerID;references:ID"`
	Shares     []Share `json:"-" gorm:"foreignKey:FileID"`
	SharedWith int64   `json:"sharedWith" gorm:"-"`
	ParentName string  `json:"parentName,omitempty" gorm:"-"`
}
