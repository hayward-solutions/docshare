package handlers

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/internal/storage"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/previewtoken"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FilesHandler struct {
	DB             *gorm.DB
	Storage        *storage.MinIOClient
	Access         *services.AccessService
	PreviewService *services.PreviewService
	Audit          *services.AuditService
}

func NewFilesHandler(db *gorm.DB, storageClient *storage.MinIOClient, access *services.AccessService, preview *services.PreviewService, audit *services.AuditService) *FilesHandler {
	return &FilesHandler{DB: db, Storage: storageClient, Access: access, PreviewService: preview, Audit: audit}
}

func (h *FilesHandler) Upload(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "file is required")
	}

	var parentID *uuid.UUID
	parentIDRaw := strings.TrimSpace(c.FormValue("parentID"))
	if parentIDRaw != "" {
		parsed, parseErr := parseUUID(parentIDRaw)
		if parseErr != nil {
			return utils.Error(c, fiber.StatusBadRequest, "invalid parentID")
		}
		parentID = &parsed

		var parent models.File
		if err := h.DB.First(&parent, "id = ?", parsed).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.Error(c, fiber.StatusNotFound, "parent folder not found")
			}
			return utils.Error(c, fiber.StatusInternalServerError, "failed validating parent folder")
		}
		if !parent.IsDirectory {
			return utils.Error(c, fiber.StatusBadRequest, "parentID must be a directory")
		}
		if !h.Access.HasAccess(c.Context(), currentUser.ID, parent.ID, models.SharePermissionEdit) {
			logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
				"action":              "file_upload",
				"target_id":           parent.ID.String(),
				"required_permission": "edit",
			})
			return utils.Error(c, fiber.StatusForbidden, "no permission to upload to parent directory")
		}
	}

	stream, err := fileHeader.Open()
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed opening uploaded file")
	}
	defer stream.Close()

	filename := filepath.Base(strings.TrimSpace(fileHeader.Filename))
	if filename == "" {
		return utils.Error(c, fiber.StatusBadRequest, "invalid filename")
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	objectName := fmt.Sprintf("%s/%s/%s", currentUser.ID.String(), uuid.New().String(), filename)
	if err := h.Storage.Upload(c.Context(), objectName, stream, fileHeader.Size, contentType); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed uploading file")
	}

	entry := models.File{
		Name:        filename,
		MimeType:    contentType,
		Size:        fileHeader.Size,
		IsDirectory: false,
		ParentID:    parentID,
		OwnerID:     currentUser.ID,
		StoragePath: objectName,
	}

	if err := h.DB.Create(&entry).Error; err != nil {
		_ = h.Storage.Delete(c.Context(), objectName)
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating file record")
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_uploaded", map[string]interface{}{
		"file_id":      entry.ID.String(),
		"file_name":    filename,
		"file_size":    fileHeader.Size,
		"mime_type":    contentType,
		"storage_path": objectName,
		"parent_id":    parentID,
	})

	auditDetails := map[string]interface{}{
		"file_name": filename,
		"file_size": fileHeader.Size,
		"mime_type": contentType,
	}
	if parentID != nil {
		auditDetails["parent_id"] = parentID.String()
	}
	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.upload",
		ResourceType: "file",
		ResourceID:   &entry.ID,
		Details:      auditDetails,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, entry)
}

type createDirectoryRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentID"`
}

func (h *FilesHandler) CreateDirectory(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req createDirectoryRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return utils.Error(c, fiber.StatusBadRequest, "name is required")
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		parsed, err := parseUUID(*req.ParentID)
		if err != nil {
			return utils.Error(c, fiber.StatusBadRequest, "invalid parentID")
		}
		parentID = &parsed

		var parent models.File
		if err := h.DB.First(&parent, "id = ?", parsed).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.Error(c, fiber.StatusNotFound, "parent folder not found")
			}
			return utils.Error(c, fiber.StatusInternalServerError, "failed loading parent")
		}
		if !parent.IsDirectory {
			return utils.Error(c, fiber.StatusBadRequest, "parentID must be a directory")
		}
		if !h.Access.HasAccess(c.Context(), currentUser.ID, parent.ID, models.SharePermissionEdit) {
			return utils.Error(c, fiber.StatusForbidden, "no permission to create in parent directory")
		}
	}

	dir := models.File{
		Name:        name,
		MimeType:    "inode/directory",
		Size:        0,
		IsDirectory: true,
		ParentID:    parentID,
		OwnerID:     currentUser.ID,
		StoragePath: "",
	}

	if err := h.DB.Create(&dir).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating directory")
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "folder.create",
		ResourceType: "file",
		ResourceID:   &dir.ID,
		Details: map[string]interface{}{
			"folder_name": name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, dir)
}

func (h *FilesHandler) ListRoot(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var owned []models.File
	if err := h.DB.Preload("Owner").Where("owner_id = ? AND parent_id IS NULL", currentUser.ID).Order("created_at DESC").Find(&owned).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed listing owned files")
	}

	var shared []models.File
	if err := h.DB.
		Preload("Owner").
		Table("files").
		Distinct("files.*").
		Joins("LEFT JOIN shares ON shares.file_id = files.id").
		Joins("LEFT JOIN group_memberships gm ON gm.group_id = shares.shared_with_group_id").
		Where("files.parent_id IS NULL").
		Where("files.owner_id <> ?", currentUser.ID).
		Where("shares.expires_at IS NULL OR shares.expires_at > NOW()").
		Where("shares.shared_with_user_id = ? OR gm.user_id = ?", currentUser.ID, currentUser.ID).
		Find(&shared).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed listing shared files")
	}

	combined := make([]models.File, 0, len(owned)+len(shared))
	seen := map[uuid.UUID]bool{}
	for _, item := range owned {
		if !seen[item.ID] {
			combined = append(combined, item)
			seen[item.ID] = true
		}
	}
	for _, item := range shared {
		if !seen[item.ID] {
			combined = append(combined, item)
			seen[item.ID] = true
		}
	}

	if len(combined) > 0 {
		fileIDs := make([]uuid.UUID, len(combined))
		for i, f := range combined {
			fileIDs[i] = f.ID
		}

		var results []struct {
			FileID uuid.UUID
			Count  int64
		}
		h.DB.Model(&models.Share{}).
			Select("file_id, count(*) as count").
			Where("file_id IN ?", fileIDs).
			Group("file_id").
			Scan(&results)

		counts := make(map[uuid.UUID]int64)
		for _, r := range results {
			counts[r.FileID] = r.Count
		}

		for i := range combined {
			combined[i].SharedWith = counts[combined[i].ID]
		}
	}

	return utils.Success(c, fiber.StatusOK, combined)
}

func (h *FilesHandler) Get(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	var file models.File
	if err := h.DB.Preload("Owner").First(&file, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	return utils.Success(c, fiber.StatusOK, file)
}

func (h *FilesHandler) ListChildren(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	var parent models.File
	if err := h.DB.First(&parent, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "directory not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading directory")
	}
	if !parent.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "file is not a directory")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, parent.ID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	var children []models.File
	if err := h.DB.Preload("Owner").Where("parent_id = ?", parent.ID).Order("is_directory DESC, name ASC").Find(&children).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading children")
	}

	if len(children) > 0 {
		fileIDs := make([]uuid.UUID, len(children))
		for i, f := range children {
			fileIDs[i] = f.ID
		}

		var results []struct {
			FileID uuid.UUID
			Count  int64
		}
		h.DB.Model(&models.Share{}).
			Select("file_id, count(*) as count").
			Where("file_id IN ?", fileIDs).
			Group("file_id").
			Scan(&results)

		counts := make(map[uuid.UUID]int64)
		for _, r := range results {
			counts[r.FileID] = r.Count
		}

		for i := range children {
			children[i].SharedWith = counts[children[i].ID]
		}
	}

	return utils.Success(c, fiber.StatusOK, children)
}

func (h *FilesHandler) Download(c *fiber.Ctx) error {
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
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot download a directory")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionDownload) {
		logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
			"action":              "file_download",
			"target_id":           file.ID.String(),
			"file_name":           file.Name,
			"required_permission": "download",
		})
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	obj, err := h.Storage.Download(c.Context(), file.StoragePath)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed downloading file")
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return utils.Error(c, fiber.StatusInternalServerError, "failed reading object metadata")
	}

	contentType := stat.ContentType
	if contentType == "" {
		contentType = file.MimeType
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_downloaded", map[string]interface{}{
		"file_id":   file.ID.String(),
		"file_name": file.Name,
		"file_size": file.Size,
		"mime_type": file.MimeType,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.download",
		ResourceType: "file",
		ResourceID:   &file.ID,
		Details: map[string]interface{}{
			"file_name": file.Name,
			"file_size": file.Size,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
	return c.SendStream(obj, int(stat.Size))
}

func (h *FilesHandler) PreviewURL(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	token := previewtoken.Generate(fileID.String(), currentUser.ID.String())

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"path":  "/api/files/" + fileID.String() + "/proxy",
		"token": token,
	})
}

func (h *FilesHandler) ProxyPreview(c *fiber.Ctx) error {
	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	var currentUser *models.User
	previewToken := c.Query("token")

	if previewToken != "" {
		if previewtoken.IsUsed(previewToken) {
			return utils.Error(c, fiber.StatusUnauthorized, "token already used")
		}
		tokenFileID, tokenUserID, err := previewtoken.GetMetadata(previewToken)
		if err == nil && tokenFileID == fileID.String() {
			var user models.User
			if dbErr := h.DB.First(&user, "id = ?", tokenUserID).Error; dbErr == nil {
				currentUser = &user
			}
		}
	} else {
		currentUser = middleware.GetCurrentUser(c)
	}

	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var file models.File
	if err := h.DB.First(&file, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot preview a directory")
	}
	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	if previewToken != "" {
		previewtoken.MarkUsed(previewToken)
	}

	obj, err := h.Storage.Download(c.Context(), file.StoragePath)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed downloading file")
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return utils.Error(c, fiber.StatusInternalServerError, "failed reading object metadata")
	}

	contentType := stat.ContentType
	if contentType == "" {
		contentType = file.MimeType
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "inline")
	return c.SendStream(obj, int(stat.Size))
}

func (h *FilesHandler) DownloadURL(c *fiber.Ctx) error {
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
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot download a directory")
	}
	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionDownload) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"url": "/api/files/" + fileID.String() + "/download",
	})
}

func (h *FilesHandler) ConvertPreview(c *fiber.Ctx) error {
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

	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	url, err := h.PreviewService.ConvertToPreview(c.Context(), &file)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed converting preview")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"url": url})
}

func (h *FilesHandler) Search(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 {
		return utils.Error(c, fiber.StatusBadRequest, "search query must be at least 2 characters")
	}

	searchValue := "%" + strings.ToLower(q) + "%"
	directoryIDRaw := strings.TrimSpace(c.Query("directoryID"))

	var files []models.File

	if directoryIDRaw != "" {
		dirID, err := parseUUID(directoryIDRaw)
		if err != nil {
			return utils.Error(c, fiber.StatusBadRequest, "invalid directoryID")
		}

		var dir models.File
		if err := h.DB.First(&dir, "id = ?", dirID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.Error(c, fiber.StatusNotFound, "directory not found")
			}
			return utils.Error(c, fiber.StatusInternalServerError, "failed loading directory")
		}
		if !dir.IsDirectory {
			return utils.Error(c, fiber.StatusBadRequest, "specified ID is not a directory")
		}
		if !h.Access.HasAccess(c.Context(), currentUser.ID, dir.ID, models.SharePermissionView) {
			return utils.Error(c, fiber.StatusForbidden, "access denied")
		}

		var descendantIDs []struct{ ID uuid.UUID }
		if err := h.DB.Raw(`
			WITH RECURSIVE descendants AS (
				SELECT id FROM files WHERE id = ? AND deleted_at IS NULL
				UNION ALL
				SELECT f.id FROM files f
				INNER JOIN descendants d ON f.parent_id = d.id
				WHERE f.deleted_at IS NULL
			)
			SELECT id FROM descendants WHERE id != ?
		`, dirID, dirID).Scan(&descendantIDs).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}

		if len(descendantIDs) == 0 {
			return utils.Success(c, fiber.StatusOK, []models.File{})
		}

		ids := make([]uuid.UUID, len(descendantIDs))
		for i, d := range descendantIDs {
			ids[i] = d.ID
		}

		if err := h.DB.Preload("Owner").
			Where("id IN ? AND LOWER(name) LIKE ?", ids, searchValue).
			Order("is_directory DESC, name ASC").
			Limit(50).
			Find(&files).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}
	} else {
		if err := h.DB.Preload("Owner").
			Where("owner_id = ? AND LOWER(name) LIKE ?", currentUser.ID, searchValue).
			Order("is_directory DESC, name ASC").
			Limit(50).
			Find(&files).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}
	}

	h.enrichParentNames(files)

	return utils.Success(c, fiber.StatusOK, files)
}

func (h *FilesHandler) enrichParentNames(files []models.File) {
	parentIDs := make([]uuid.UUID, 0)
	for _, f := range files {
		if f.ParentID != nil {
			parentIDs = append(parentIDs, *f.ParentID)
		}
	}
	if len(parentIDs) == 0 {
		return
	}

	var parents []models.File
	h.DB.Select("id", "name").Where("id IN ?", parentIDs).Find(&parents)

	parentMap := make(map[uuid.UUID]string)
	for _, p := range parents {
		parentMap[p.ID] = p.Name
	}

	for i := range files {
		if files[i].ParentID != nil {
			if name, ok := parentMap[*files[i].ParentID]; ok {
				files[i].ParentName = name
			}
		}
	}
}

type updateFileRequest struct {
	Name     *string `json:"name"`
	ParentID *string `json:"parentID"`
}

func (h *FilesHandler) Update(c *fiber.Ctx) error {
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

	canEdit := file.OwnerID == currentUser.ID || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionEdit)
	if !canEdit {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	var req updateFileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return utils.Error(c, fiber.StatusBadRequest, "name cannot be empty")
		}
		updates["name"] = name
	}

	if req.ParentID != nil {
		trimmed := strings.TrimSpace(*req.ParentID)
		if trimmed == "" {
			updates["parent_id"] = nil
		} else {
			newParentID, parseErr := parseUUID(trimmed)
			if parseErr != nil {
				return utils.Error(c, fiber.StatusBadRequest, "invalid parentID")
			}
			if newParentID == file.ID {
				return utils.Error(c, fiber.StatusBadRequest, "file cannot be parent of itself")
			}

			var newParent models.File
			if err := h.DB.First(&newParent, "id = ?", newParentID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return utils.Error(c, fiber.StatusNotFound, "new parent not found")
				}
				return utils.Error(c, fiber.StatusInternalServerError, "failed loading new parent")
			}
			if !newParent.IsDirectory {
				return utils.Error(c, fiber.StatusBadRequest, "new parent must be a directory")
			}
			if !h.Access.HasAccess(c.Context(), currentUser.ID, newParent.ID, models.SharePermissionEdit) {
				return utils.Error(c, fiber.StatusForbidden, "no permission for target directory")
			}
			if file.IsDirectory {
				isChild, checkErr := h.isDescendant(file.ID, newParent.ID)
				if checkErr != nil {
					return utils.Error(c, fiber.StatusInternalServerError, "failed validating move")
				}
				if isChild {
					return utils.Error(c, fiber.StatusBadRequest, "cannot move directory inside itself")
				}
			}

			updates["parent_id"] = newParentID
		}
	}

	if len(updates) == 0 {
		return utils.Error(c, fiber.StatusBadRequest, "no valid fields to update")
	}

	if err := h.DB.Model(&models.File{}).Where("id = ?", file.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating file")
	}

	var updated models.File
	if err := h.DB.First(&updated, "id = ?", file.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading updated file")
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.update",
		ResourceType: "file",
		ResourceID:   &file.ID,
		Details: map[string]interface{}{
			"file_name": updated.Name,
			"changes":   updates,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, updated)
}

func (h *FilesHandler) Delete(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionEdit) {
		logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
			"action":              "file_delete",
			"target_id":           fileID.String(),
			"required_permission": "edit",
		})
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	var file models.File
	if err := h.DB.Select("id", "name", "is_directory").First(&file, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}

	var shareRecipientIDs []string
	var shares []models.Share
	h.DB.Where("file_id = ?", fileID).Where("expires_at IS NULL OR expires_at > NOW()").Find(&shares)
	seen := map[uuid.UUID]bool{currentUser.ID: true}
	for _, share := range shares {
		if share.SharedWithUserID != nil && !seen[*share.SharedWithUserID] {
			seen[*share.SharedWithUserID] = true
			shareRecipientIDs = append(shareRecipientIDs, share.SharedWithUserID.String())
		}
	}

	if err := h.deleteRecursive(c.Context(), fileID); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed deleting file")
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_deleted", map[string]interface{}{
		"file_id": fileID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.delete",
		ResourceType: "file",
		ResourceID:   &fileID,
		Details: map[string]interface{}{
			"file_name":       file.Name,
			"is_directory":    file.IsDirectory,
			"notify_user_ids": shareRecipientIDs,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "file deleted"})
}

func (h *FilesHandler) Path(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	path := make([]models.File, 0)
	current := fileID
	for {
		var file models.File
		if err := h.DB.First(&file, "id = ?", current).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return utils.Error(c, fiber.StatusInternalServerError, "failed building breadcrumb path")
		}

		path = append(path, file)
		if file.ParentID == nil {
			break
		}
		current = *file.ParentID
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return utils.Success(c, fiber.StatusOK, path)
}

func (h *FilesHandler) deleteRecursive(ctx context.Context, fileID uuid.UUID) error {
	var file models.File
	if err := h.DB.First(&file, "id = ?", fileID).Error; err != nil {
		return err
	}

	if file.IsDirectory {
		var children []models.File
		if err := h.DB.Where("parent_id = ?", file.ID).Find(&children).Error; err != nil {
			return err
		}
		for _, child := range children {
			if err := h.deleteRecursive(ctx, child.ID); err != nil {
				return err
			}
		}
	} else if file.StoragePath != "" {
		if err := h.Storage.Delete(ctx, file.StoragePath); err != nil {
			return err
		}
		if file.ThumbnailPath != nil && *file.ThumbnailPath != "" {
			_ = h.Storage.Delete(ctx, *file.ThumbnailPath)
		}
	}

	if err := h.DB.Where("file_id = ?", file.ID).Delete(&models.Share{}).Error; err != nil {
		return err
	}

	return h.DB.Delete(&models.File{}, "id = ?", file.ID).Error
}

func (h *FilesHandler) PublicGet(c *fiber.Ctx) error {
	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	currentUser := middleware.GetCurrentUser(c)
	isLoggedIn := currentUser != nil

	if isLoggedIn {
		if h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionView) {
			var file models.File
			if err := h.DB.Preload("Owner").First(&file, "id = ?", fileID).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return utils.Error(c, fiber.StatusNotFound, "file not found")
				}
				return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
			}
			return utils.Success(c, fiber.StatusOK, file)
		}
	}

	shareType := h.Access.GetPublicShareType(c.Context(), fileID)
	if shareType == nil {
		return utils.Error(c, fiber.StatusNotFound, "file not found")
	}

	if *shareType == models.ShareTypePublicLoggedIn && !isLoggedIn {
		return utils.Error(c, fiber.StatusUnauthorized, "login required to access this file")
	}

	var file models.File
	if err := h.DB.Preload("Owner").First(&file, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}

	return utils.Success(c, fiber.StatusOK, file)
}

func (h *FilesHandler) PublicDownload(c *fiber.Ctx) error {
	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	currentUser := middleware.GetCurrentUser(c)
	isLoggedIn := currentUser != nil

	if isLoggedIn && h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionDownload) {
		return h.downloadFile(c, fileID)
	}

	requireLogin := false
	if !h.Access.HasPublicAccess(c.Context(), fileID, models.SharePermissionDownload, false) {
		requireLogin = true
		if !h.Access.HasPublicAccess(c.Context(), fileID, models.SharePermissionDownload, true) {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
	}

	if requireLogin && !isLoggedIn {
		return utils.Error(c, fiber.StatusUnauthorized, "login required to access this file")
	}

	return h.downloadFile(c, fileID)
}

func (h *FilesHandler) PublicChildren(c *fiber.Ctx) error {
	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	currentUser := middleware.GetCurrentUser(c)
	isLoggedIn := currentUser != nil

	shareType := h.Access.GetPublicShareType(c.Context(), fileID)
	hasPrivateAccess := isLoggedIn && h.Access.HasAccess(c.Context(), currentUser.ID, fileID, models.SharePermissionView)

	if shareType == nil && !hasPrivateAccess {
		return utils.Error(c, fiber.StatusNotFound, "directory not found")
	}

	if shareType != nil && *shareType == models.ShareTypePublicLoggedIn && !isLoggedIn && !hasPrivateAccess {
		return utils.Error(c, fiber.StatusUnauthorized, "login required to access this directory")
	}

	var parent models.File
	if err := h.DB.First(&parent, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "directory not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading directory")
	}
	if !parent.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "file is not a directory")
	}

	var children []models.File
	if err := h.DB.Preload("Owner").Where("parent_id = ?", parent.ID).Order("is_directory DESC, name ASC").Find(&children).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading children")
	}

	return utils.Success(c, fiber.StatusOK, children)
}

func (h *FilesHandler) downloadFile(c *fiber.Ctx, fileID uuid.UUID) error {
	var file models.File
	if err := h.DB.First(&file, "id = ?", fileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot download a directory")
	}

	obj, err := h.Storage.Download(c.Context(), file.StoragePath)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed downloading file")
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return utils.Error(c, fiber.StatusInternalServerError, "failed reading object metadata")
	}

	contentType := stat.ContentType
	if contentType == "" {
		contentType = file.MimeType
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
	return c.SendStream(obj, int(stat.Size))
}

func (h *FilesHandler) isDescendant(ancestorID, candidateChildID uuid.UUID) (bool, error) {
	current := candidateChildID
	for {
		if current == ancestorID {
			return true, nil
		}

		var file models.File
		err := h.DB.Select("id", "parent_id").First(&file, "id = ?", current).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}
			return false, err
		}
		if file.ParentID == nil {
			return false, nil
		}
		current = *file.ParentID
	}
}
