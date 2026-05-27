package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docshare/api/internal/middleware"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/internal/services"
	"github.com/docshare/api/pkg/logger"
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// editableContentMaxBytes caps in-app text-document reads and saves. The
// editor flow exists for human-authored docs (markdown, txt, code), not for
// arbitrary blobs — keeping this bounded avoids loading multi-GB binaries
// into a Tiptap session and blocks the save path from being used to push
// large bodies through the JSON API.
const editableContentMaxBytes = 5 * 1024 * 1024

// editableBinaryMaxBytes is the analogous cap for the binary editor path
// (spreadsheet workbooks). A Univer session decoding a multi-hundred-MB
// XLSX would crash the tab; a smaller cap also keeps the in-place PUT
// path from being misused to ship large bodies.
const editableBinaryMaxBytes = 10 * 1024 * 1024

// isEditableTextMime gates the JSON /content endpoints. Anything that's not
// human-readable text is rejected so a save through this path can't silently
// overwrite a binary at file.StoragePath with text.
func isEditableTextMime(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	switch mimeType {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/typescript",
		"application/x-yaml",
		"application/yaml":
		return true
	}
	return false
}

// isEditableSpreadsheetBinaryMime gates the /binary endpoints to the
// workbook formats the SpreadsheetEditor knows how to parse and re-emit
// (XLSX, XLS, ODS). text/csv stays on the text path because it's plain text.
func isEditableSpreadsheetBinaryMime(mimeType string) bool {
	switch mimeType {
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel",
		"application/vnd.oasis.opendocument.spreadsheet":
		return true
	}
	return false
}

// isCreatableDocMime is the union — what CreateDoc is willing to mint a
// fresh blank file for. Editable text + editable binary spreadsheets.
func isCreatableDocMime(mimeType string) bool {
	return isEditableTextMime(mimeType) || isEditableSpreadsheetBinaryMime(mimeType)
}

// GetContent streams the raw bytes of an editable text file as JSON so the
// browser editor can populate without going through the preview-token /
// blob-URL dance the read-only viewer uses.
func (h *FilesHandler) GetContent(c *fiber.Ctx) error {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot read directory content")
	}
	if !isEditableTextMime(file.MimeType) {
		return utils.Error(c, fiber.StatusUnsupportedMediaType, "file type is not editable as text")
	}
	if file.Size > editableContentMaxBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds editor maximum of %d bytes", editableContentMaxBytes))
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	obj, err := h.Storage.Download(c.Context(), file.StoragePath)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed downloading file")
	}
	defer obj.Close()

	body, err := io.ReadAll(io.LimitReader(obj, editableContentMaxBytes+1))
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed reading file content")
	}
	if int64(len(body)) > editableContentMaxBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, "file exceeds editor maximum")
	}

	canEdit := file.OwnerID == currentUser.ID || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionEdit)

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"content":  string(body),
		"mimeType": file.MimeType,
		"name":     file.Name,
		"size":     file.Size,
		"canEdit":  canEdit,
	})
}

type saveContentRequest struct {
	Content string `json:"content"`
}

// SaveContent overwrites the S3 object backing the file with the supplied
// text body and refreshes Size in the DB. This is intentionally in-place: v1
// has no version history, so a save replaces the bytes at file.StoragePath.
func (h *FilesHandler) SaveContent(c *fiber.Ctx) error {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot save content to a directory")
	}
	if !isEditableTextMime(file.MimeType) {
		return utils.Error(c, fiber.StatusUnsupportedMediaType, "file type is not editable as text")
	}

	canEdit := file.OwnerID == currentUser.ID || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionEdit)
	if !canEdit {
		logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
			"action":              "file_edit_save",
			"target_id":           file.ID.String(),
			"required_permission": "edit",
		})
		return utils.Error(c, fiber.StatusForbidden, "no permission to edit this file")
	}

	var req saveContentRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	body := []byte(req.Content)
	if int64(len(body)) > editableContentMaxBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("content exceeds editor maximum of %d bytes", editableContentMaxBytes))
	}

	if err := h.Storage.Upload(c.Context(), file.StoragePath, bytes.NewReader(body), int64(len(body)), file.MimeType); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed saving file content")
	}

	updates := map[string]interface{}{"size": int64(len(body))}
	if err := h.DB.Model(&models.File{}).Where("id = ?", file.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating file metadata")
	}

	var updated models.File
	if err := h.DB.Preload("Owner").First(&updated, "id = ?", file.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading updated file")
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_content_saved", map[string]interface{}{
		"file_id":   updated.ID.String(),
		"file_name": updated.Name,
		"file_size": updated.Size,
		"mime_type": updated.MimeType,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.edit",
		ResourceType: "file",
		ResourceID:   &updated.ID,
		Details: map[string]interface{}{
			"file_name": updated.Name,
			"file_size": updated.Size,
			"mime_type": updated.MimeType,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, updated)
}

type createDocRequest struct {
	Name     string  `json:"name"`
	MimeType string  `json:"mimeType"`
	ParentID *string `json:"parentID"`
}

// CreateDoc creates a zero-byte text document directly (no presign roundtrip)
// so the new-document menu can hand the user a fresh file id to open in the
// editor. The bytes get written on first Save.
func (h *FilesHandler) CreateDoc(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req createDocRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	filename := filepath.Base(strings.TrimSpace(req.Name))
	if filename == "" || filename == "." || filename == "/" {
		return utils.Error(c, fiber.StatusBadRequest, "invalid filename")
	}

	contentType := resolveMimeType(filename, strings.TrimSpace(req.MimeType))
	if !isCreatableDocMime(contentType) {
		return utils.Error(c, fiber.StatusBadRequest, "mime type is not a supported document type")
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		parsed, parseErr := parseUUID(strings.TrimSpace(*req.ParentID))
		if parseErr != nil {
			return utils.Error(c, fiber.StatusBadRequest, "invalid parentID")
		}
		var parent models.File
		if err := h.DB.First(&parent, "id = ?", parsed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.Error(c, fiber.StatusNotFound, "parent folder not found")
			}
			return utils.Error(c, fiber.StatusInternalServerError, "failed validating parent folder")
		}
		if !parent.IsDirectory {
			return utils.Error(c, fiber.StatusBadRequest, "parentID must be a directory")
		}
		if !h.Access.HasAccess(c.Context(), currentUser.ID, parent.ID, models.SharePermissionEdit) {
			return utils.Error(c, fiber.StatusForbidden, "no permission to create in parent directory")
		}
		parentID = &parent.ID
	}

	objectName := fmt.Sprintf("%s/%s/%s", currentUser.ID.String(), uuid.New().String(), filename)
	if err := h.Storage.Upload(c.Context(), objectName, bytes.NewReader(nil), 0, contentType); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating file object")
	}

	entry := models.File{
		Name:        filename,
		MimeType:    contentType,
		Size:        0,
		IsDirectory: false,
		ParentID:    parentID,
		OwnerID:     currentUser.ID,
		StoragePath: objectName,
	}

	if err := h.DB.Create(&entry).Error; err != nil {
		_ = h.Storage.Delete(c.Context(), objectName)
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating file record")
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_created", map[string]interface{}{
		"file_id":      entry.ID.String(),
		"file_name":    filename,
		"mime_type":    contentType,
		"storage_path": objectName,
		"parent_id":    parentID,
		"source":       "editor_new",
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.create",
		ResourceType: "file",
		ResourceID:   &entry.ID,
		Details: map[string]interface{}{
			"file_name": filename,
			"mime_type": contentType,
			"source":    "editor_new",
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, entry)
}

// GetBinary streams the raw bytes of a spreadsheet-editable file with
// view permission. Sister of GetContent for formats that can't safely
// round-trip through a JSON string body (XLSX, XLS, ODS). The 0-byte
// case is allowed so the editor can seed a fresh workbook for files
// just minted by CreateDoc.
func (h *FilesHandler) GetBinary(c *fiber.Ctx) error {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot read directory content")
	}
	if !isEditableSpreadsheetBinaryMime(file.MimeType) {
		return utils.Error(c, fiber.StatusUnsupportedMediaType, "file type is not editable as a binary workbook")
	}
	if file.Size > editableBinaryMaxBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds editor maximum of %d bytes", editableBinaryMaxBytes))
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionView) {
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	// A freshly-minted blank file from CreateDoc has size 0 — stream nothing
	// rather than hitting S3 for an empty object, and let the client treat
	// the missing body as "open an empty workbook."
	if file.Size == 0 {
		c.Set("Content-Type", file.MimeType)
		return c.SendStream(bytes.NewReader(nil), 0)
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

	// No defer obj.Close() — SendStream is responsible for the reader
	// lifecycle. A defer here closes the object before Fiber starts
	// reading the body and the response ends up as the MinIO
	// "Object is already closed" error message.
	c.Set("Content-Type", file.MimeType)
	c.Set("Content-Disposition", "inline")
	return c.SendStream(obj, int(stat.Size))
}

// SaveBinary overwrites the S3 object backing the file with the supplied
// raw bytes. Edit permission required, capped at editableBinaryMaxBytes.
// Used by the spreadsheet editor to push freshly-emitted XLSX bytes back.
func (h *FilesHandler) SaveBinary(c *fiber.Ctx) error {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot save content to a directory")
	}
	if !isEditableSpreadsheetBinaryMime(file.MimeType) {
		return utils.Error(c, fiber.StatusUnsupportedMediaType, "file type is not editable as a binary workbook")
	}

	canEdit := file.OwnerID == currentUser.ID || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionEdit)
	if !canEdit {
		logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
			"action":              "file_edit_save_binary",
			"target_id":           file.ID.String(),
			"required_permission": "edit",
		})
		return utils.Error(c, fiber.StatusForbidden, "no permission to edit this file")
	}

	body := c.Body()
	if int64(len(body)) > editableBinaryMaxBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("content exceeds editor maximum of %d bytes", editableBinaryMaxBytes))
	}

	if err := h.Storage.Upload(c.Context(), file.StoragePath, bytes.NewReader(body), int64(len(body)), file.MimeType); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed saving file content")
	}

	if err := h.DB.Model(&models.File{}).Where("id = ?", file.ID).Updates(map[string]interface{}{"size": int64(len(body))}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating file metadata")
	}

	var updated models.File
	if err := h.DB.Preload("Owner").First(&updated, "id = ?", file.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading updated file")
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_content_saved", map[string]interface{}{
		"file_id":   updated.ID.String(),
		"file_name": updated.Name,
		"file_size": updated.Size,
		"mime_type": updated.MimeType,
		"mode":      "binary",
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.edit",
		ResourceType: "file",
		ResourceID:   &updated.ID,
		Details: map[string]interface{}{
			"file_name": updated.Name,
			"file_size": updated.Size,
			"mime_type": updated.MimeType,
			"mode":      "binary",
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, updated)
}
