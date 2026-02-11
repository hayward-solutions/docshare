package services

import (
	"context"
	"time"

	"github.com/docshare/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AccessService struct {
	DB *gorm.DB
}

func NewAccessService(db *gorm.DB) *AccessService {
	return &AccessService{DB: db}
}

func (a *AccessService) HasAccess(ctx context.Context, userID uuid.UUID, fileID uuid.UUID, requiredPermission models.SharePermission) bool {
	requiredLevel, ok := permissionLevel(requiredPermission)
	if !ok {
		return false
	}

	currentID := fileID
	now := time.Now()

	for {
		var file models.File
		err := a.DB.WithContext(ctx).First(&file, "id = ?", currentID).Error
		if err != nil {
			return false
		}

		if file.OwnerID == userID {
			return true
		}

		var directShares []models.Share
		if err := a.DB.WithContext(ctx).
			Where("file_id = ? AND shared_with_user_id = ?", currentID, userID).
			Where("expires_at IS NULL OR expires_at > ?", now).
			Find(&directShares).Error; err == nil {
			for _, share := range directShares {
				if lvl, exists := permissionLevel(share.Permission); exists && lvl >= requiredLevel {
					return true
				}
			}
		}

		var groupShares []models.Share
		if err := a.DB.WithContext(ctx).
			Table("shares").
			Joins("JOIN group_memberships ON group_memberships.group_id = shares.shared_with_group_id AND group_memberships.user_id = ?", userID).
			Where("shares.file_id = ?", currentID).
			Where("shares.expires_at IS NULL OR shares.expires_at > ?", now).
			Select("shares.*").
			Scan(&groupShares).Error; err == nil {
			for _, share := range groupShares {
				if lvl, exists := permissionLevel(share.Permission); exists && lvl >= requiredLevel {
					return true
				}
			}
		}

		if file.ParentID == nil {
			break
		}
		currentID = *file.ParentID
	}

	return false
}

func permissionLevel(permission models.SharePermission) (int, bool) {
	switch permission {
	case models.SharePermissionView:
		return 1, true
	case models.SharePermissionDownload:
		return 2, true
	case models.SharePermissionEdit:
		return 3, true
	default:
		return 0, false
	}
}
