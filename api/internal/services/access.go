package services

import (
	"context"
	"time"

	"github.com/docshare/api/internal/models"
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
			Where("share_type = ?", models.ShareTypePrivate).
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
			Where("shares.share_type = ?", models.ShareTypePrivate).
			Where("shares.expires_at IS NULL OR shares.expires_at > ?", now).
			Select("shares.*").
			Scan(&groupShares).Error; err == nil {
			for _, share := range groupShares {
				if lvl, exists := permissionLevel(share.Permission); exists && lvl >= requiredLevel {
					return true
				}
			}
		}

		var publicShares []models.Share
		if err := a.DB.WithContext(ctx).
			Where("file_id = ? AND share_type IN ?", currentID, []models.ShareType{models.ShareTypePublicAnyone, models.ShareTypePublicLoggedIn}).
			Where("expires_at IS NULL OR expires_at > ?", now).
			Find(&publicShares).Error; err == nil {
			for _, share := range publicShares {
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

func (a *AccessService) HasPublicAccess(ctx context.Context, fileID uuid.UUID, requiredPermission models.SharePermission, requireLogin bool) bool {
	requiredLevel, ok := permissionLevel(requiredPermission)
	if !ok {
		return false
	}

	shareTypes := []models.ShareType{models.ShareTypePublicAnyone}
	if requireLogin {
		shareTypes = append(shareTypes, models.ShareTypePublicLoggedIn)
	}

	currentID := fileID
	now := time.Now()

	for {
		var file models.File
		err := a.DB.WithContext(ctx).First(&file, "id = ?", currentID).Error
		if err != nil {
			return false
		}

		var shares []models.Share
		if err := a.DB.WithContext(ctx).
			Where("file_id = ? AND share_type IN ?", currentID, shareTypes).
			Where("expires_at IS NULL OR expires_at > ?", now).
			Find(&shares).Error; err == nil {
			for _, share := range shares {
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

func (a *AccessService) GetPublicShareType(ctx context.Context, fileID uuid.UUID) *models.ShareType {
	now := time.Now()
	currentID := fileID

	for {
		var file models.File
		if err := a.DB.WithContext(ctx).First(&file, "id = ?", currentID).Error; err != nil {
			return nil
		}

		var share models.Share
		if err := a.DB.WithContext(ctx).
			Where("file_id = ? AND share_type IN ?", currentID, []models.ShareType{models.ShareTypePublicAnyone, models.ShareTypePublicLoggedIn}).
			Where("expires_at IS NULL OR expires_at > ?", now).
			Order("CASE WHEN share_type = 'public_anyone' THEN 0 ELSE 1 END").
			First(&share).Error; err == nil {
			return &share.ShareType
		}

		if file.ParentID == nil {
			break
		}
		currentID = *file.ParentID
	}

	return nil
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
