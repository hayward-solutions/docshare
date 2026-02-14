package handlers

import (
	"strings"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SharesHandler struct {
	DB     *gorm.DB
	Access *services.AccessService
	Audit  *services.AuditService
}

func NewSharesHandler(db *gorm.DB, access *services.AccessService, audit *services.AuditService) *SharesHandler {
	return &SharesHandler{DB: db, Access: access, Audit: audit}
}

type createShareRequest struct {
	UserID     *uuid.UUID             `json:"userID"`
	GroupID    *uuid.UUID             `json:"groupID"`
	ShareType  *models.ShareType      `json:"shareType"`
	Permission models.SharePermission `json:"permission"`
	ExpiresAt  *time.Time             `json:"expiresAt"`
}

func (h *SharesHandler) ShareFile(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	var file models.File
	if err := h.DB.First(&file, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}

	if file.OwnerID != currentUser.ID {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	var req createShareRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if !isValidSharePermission(string(req.Permission)) {
		return utils.Error(c, fiber.StatusBadRequest, "invalid permission")
	}

	shareType := models.ShareTypePrivate
	if req.ShareType != nil {
		if !isValidShareType(string(*req.ShareType)) {
			return utils.Error(c, fiber.StatusBadRequest, "invalid share type")
		}
		shareType = *req.ShareType
	}

	if shareType == models.ShareTypePrivate {
		if (req.UserID == nil && req.GroupID == nil) || (req.UserID != nil && req.GroupID != nil) {
			return utils.Error(c, fiber.StatusBadRequest, "exactly one of userID or groupID is required for private shares")
		}

		if req.UserID != nil {
			if *req.UserID == currentUser.ID {
				return utils.Error(c, fiber.StatusBadRequest, "cannot share with yourself")
			}
			var targetUser models.User
			if err := h.DB.First(&targetUser, "id = ?", *req.UserID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return utils.Error(c, fiber.StatusNotFound, "target user not found")
				}
				return utils.Error(c, fiber.StatusInternalServerError, "failed loading target user")
			}
		}
		if req.GroupID != nil {
			var group models.Group
			if err := h.DB.First(&group, "id = ?", *req.GroupID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return utils.Error(c, fiber.StatusNotFound, "target group not found")
				}
				return utils.Error(c, fiber.StatusInternalServerError, "failed loading target group")
			}
		}
	} else {
		if req.UserID != nil || req.GroupID != nil {
			return utils.Error(c, fiber.StatusBadRequest, "userID and groupID must not be set for public shares")
		}

		var existingCount int64
		h.DB.Model(&models.Share{}).
			Where("file_id = ? AND share_type = ?", file.ID, shareType).
			Where("expires_at IS NULL OR expires_at > NOW()").
			Count(&existingCount)
		if existingCount > 0 {
			return utils.Error(c, fiber.StatusConflict, "a public share of this type already exists for this file")
		}
	}

	share := models.Share{
		FileID:            file.ID,
		SharedByID:        currentUser.ID,
		SharedWithUserID:  req.UserID,
		SharedWithGroupID: req.GroupID,
		ShareType:         shareType,
		Permission:        req.Permission,
		ExpiresAt:         req.ExpiresAt,
	}

	if err := h.DB.Create(&share).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating share")
	}

	details := map[string]interface{}{
		"file_id":    file.ID.String(),
		"file_name":  file.Name,
		"permission": string(req.Permission),
		"share_type": string(shareType),
		"share_id":   share.ID.String(),
	}
	if req.UserID != nil {
		details["shared_with_user_id"] = req.UserID.String()
	}
	if req.GroupID != nil {
		details["shared_with_group_id"] = req.GroupID.String()
	}
	if req.ExpiresAt != nil {
		details["expires_at"] = req.ExpiresAt
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_shared", details)

	auditDetails := map[string]interface{}{
		"file_name":  file.Name,
		"permission": string(req.Permission),
		"share_type": string(shareType),
		"share_id":   share.ID.String(),
	}
	if req.UserID != nil {
		auditDetails["shared_with_user_id"] = req.UserID.String()
	}
	if req.GroupID != nil {
		auditDetails["shared_with_group_id"] = req.GroupID.String()
		var grp models.Group
		if err := h.DB.Select("name").First(&grp, "id = ?", *req.GroupID).Error; err == nil {
			auditDetails["group_name"] = grp.Name
		}
	}
	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "share.create",
		ResourceType: "share",
		ResourceID:   &file.ID,
		Details:      auditDetails,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, share)
}

func (h *SharesHandler) ListFileShares(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	p := utils.ParsePagination(c)

	baseQuery := h.DB.Model(&models.Share{}).Where("file_id = ?", fileID)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting shares")
	}

	var shares []models.Share
	if err := utils.ApplyPagination(
		baseQuery.Preload("SharedWithUser").Preload("SharedWithGroup").Preload("SharedBy"),
		p,
	).Find(&shares).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading shares")
	}

	return utils.Paginated(c, shares, p.Page, p.Limit, total)
}

func (h *SharesHandler) DeleteShare(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	shareID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid share id")
	}

	var share models.Share
	if err := h.DB.First(&share, "id = ?", shareID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "share not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading share")
	}

	if share.SharedByID != currentUser.ID && !h.Access.HasAccess(c.Context(), currentUser.ID, share.FileID, models.SharePermissionEdit) {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	var file models.File
	h.DB.Select("id", "name").First(&file, "id = ?", share.FileID)

	if err := h.DB.Delete(&models.Share{}, "id = ?", share.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed deleting share")
	}

	deleteDetails := map[string]interface{}{
		"file_name": file.Name,
		"share_id":  share.ID.String(),
	}
	if share.SharedWithUserID != nil {
		deleteDetails["shared_with_user_id"] = share.SharedWithUserID.String()
	}
	if share.SharedWithGroupID != nil {
		deleteDetails["shared_with_group_id"] = share.SharedWithGroupID.String()
	}
	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "share.delete",
		ResourceType: "share",
		ResourceID:   &share.FileID,
		Details:      deleteDetails,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "share revoked"})
}

type updateShareRequest struct {
	Permission models.SharePermission `json:"permission"`
	ExpiresAt  *time.Time             `json:"expiresAt"`
}

func (h *SharesHandler) UpdateShare(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	shareID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid share id")
	}

	var share models.Share
	if err := h.DB.First(&share, "id = ?", shareID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "share not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading share")
	}

	if share.SharedByID != currentUser.ID && !h.Access.HasAccess(c.Context(), currentUser.ID, share.FileID, models.SharePermissionEdit) {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	var req updateShareRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if !isValidSharePermission(string(req.Permission)) {
		return utils.Error(c, fiber.StatusBadRequest, "invalid permission")
	}

	updates := map[string]interface{}{
		"permission": req.Permission,
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}

	if err := h.DB.Model(&models.Share{}).Where("id = ?", share.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating share")
	}

	if err := h.DB.First(&share, "id = ?", share.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed reloading share")
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "share.update",
		ResourceType: "share",
		ResourceID:   &share.FileID,
		Details: map[string]interface{}{
			"share_id":       share.ID.String(),
			"new_permission": string(req.Permission),
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, share)
}

func (h *SharesHandler) ListSharedWithMe(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	p := utils.ParsePagination(c)

	baseQuery := h.DB.Model(&models.File{})

	search := strings.TrimSpace(c.Query("search"))
	if search != "" {
		searchValue := "%" + strings.ToLower(search) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ?", searchValue)
	}

	sharedFilesSubquery := h.DB.
		Table("shares").
		Select("file_id").
		Joins("LEFT JOIN group_memberships gm ON gm.group_id = shares.shared_with_group_id").
		Where("shares.expires_at IS NULL OR shares.expires_at > NOW()").
		Where("shares.shared_with_user_id = ? OR gm.user_id = ?", currentUser.ID, currentUser.ID).
		Where("files.owner_id != ?", currentUser.ID)

	baseQuery = baseQuery.Where("id IN (?)", sharedFilesSubquery).Where("owner_id != ?", currentUser.ID)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting shared files")
	}

	var files []models.File
	if err := utils.ApplyPagination(baseQuery.Order("created_at DESC"), p).Find(&files).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading shared files")
	}

	return utils.Paginated(c, files, p.Page, p.Limit, total)
}
