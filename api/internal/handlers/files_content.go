package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
// editor flow exists for human-authored docs (markdown, txt, code), not
// for arbitrary blobs.
//
// The save path PUTs a JSON body ({"content":"..."}), so wire size after
// escaping can dwarf the raw content — pathological inputs of all
// quotes/backslashes inflate ~2×, and all control characters
// (`\u00XX`) inflate ~6×. The cap stays at 1 MiB so even a 6× escape
// ratio fits under the SmallBodyLimitForNonUploadRoutes middleware (8
// MiB), and a defensive raw-body check in SaveContent rejects requests
// that somehow get through with too-large wire size.
const editableContentMaxBytes = 1 * 1024 * 1024

// editableBinaryMaxBytes is the analogous cap for the binary editor path
// (spreadsheet workbooks). A Univer session decoding a multi-hundred-MB
// XLSX would crash the tab. The cap is kept below
// SmallBodyLimitForNonUploadRoutes (8 MiB) so the PUT body limit
// middleware doesn't 413 us before we reach this handler.
const editableBinaryMaxBytes = 8 * 1024 * 1024

// normalizeMime strips parameters (charset, boundary, etc.) and lowercases
// the bare type/subtype. resolveMimeType can hand us values like
// "text/csv; charset=utf-8" from Go's mime.TypeByExtension, and our
// Set/equality checks would otherwise miss them.
func normalizeMime(mimeType string) string {
	if mimeType == "" {
		return ""
	}
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}
	return strings.ToLower(strings.TrimSpace(mimeType))
}

// isEditableTextMime gates the JSON /content endpoints. Anything that's not
// human-readable text is rejected so a save through this path can't silently
// overwrite a binary at file.StoragePath with text.
func isEditableTextMime(mimeType string) bool {
	m := normalizeMime(mimeType)
	if m == "" {
		return false
	}
	if strings.HasPrefix(m, "text/") {
		return true
	}
	switch m {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/typescript",
		"application/x-yaml",
		"application/yaml",
		"application/csv":
		return true
	}
	return false
}

// isEditableSpreadsheetBinaryMime gates the /binary endpoints. Currently
// XLSX-only — the ExcelJS bridge used by the SpreadsheetEditor only parses
// and writes XLSX, so accepting .xls (BIFF) or .ods (OpenDocument) would
// either fail on load or silently rewrite the bytes as XLSX under the old
// mime/extension. text/csv stays on the text path because it's plain text.
func isEditableSpreadsheetBinaryMime(mimeType string) bool {
	return normalizeMime(mimeType) == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
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

	// Defensive raw-wire-size check. The SmallBodyLimitForNonUploadRoutes
	// middleware (8 MiB) is the primary guard, but reject obviously-too-
	// large bodies here with a clear message before parsing JSON. JSON
	// escape inflation means a < 1 MiB raw doc rarely exceeds even 2 MiB
	// wire, so cap the wire body at 6× the decoded cap.
	if int64(len(c.Body())) > editableContentMaxBytes*6 {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, "request body too large for editor save")
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

	// /binary streams the original workbook bytes to the browser, which
	// is the same exposure as /download — gate on download (or edit, or
	// owner) rather than mere view. View-only collaborators see the PDF
	// preview but should not be able to pull the unmodified file via this
	// path.
	isOwner := file.OwnerID == currentUser.ID
	canEdit := isOwner || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionEdit)
	canDownload := canEdit || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionDownload)
	if !canDownload {
		logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
			"action":              "file_binary_get",
			"target_id":           file.ID.String(),
			"required_permission": "download",
		})
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	// Surface edit-permission to the spreadsheet editor in a custom
	// response header so it can mount Univer read-only when a share
	// allows download but not edit. Mirrors the canEdit field on the
	// JSON /content response.
	c.Set("X-Can-Edit", strconv.FormatBool(canEdit))
	c.Set("Access-Control-Expose-Headers", "X-Can-Edit")

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

	// No defer obj.Close() — SendStream is responsible for the reader
	// lifecycle. A defer here closes the object before Fiber starts
	// reading the body and the response ends up as the MinIO
	// "Object is already closed" error message.
	//
	// Size comes from the DB row rather than a fresh obj.Stat() call so
	// we avoid an extra metadata round-trip; the SaveBinary path updates
	// file.Size in the same transaction as the upload, so this stays
	// in sync without a second S3 call.
	c.Set("Content-Type", file.MimeType)
	c.Set("Content-Disposition", "inline")
	return c.SendStream(obj, int(file.Size))
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

	// Snapshot the preview-job IDs that exist before we touch anything.
	// Once we bump updated_at below, an in-flight worker hits the fence
	// in ConvertToPreview, returns ErrPreviewSuperseded, and processJob
	// enqueues a fresh replacement job against the new bytes. Cleaning
	// up only the IDs we captured here leaves that replacement alone.
	var priorJobIDs []uuid.UUID
	if err := h.DB.Model(&models.PreviewJob{}).Where("file_id = ?", file.ID).Pluck("id", &priorJobIDs).Error; err != nil {
		logger.Error("preview_job_snapshot_failed", err, map[string]interface{}{
			"file_id": file.ID.String(),
		})
	}

	// Bump updated_at and clear thumbnail_path BEFORE uploading bytes.
	// This closes a race where a worker that started before us could
	// finish its render and write thumbnail_path between our upload and
	// our metadata update — at that point the fence sees the old
	// updated_at and lets the stale PDF land. By advancing updated_at
	// first, the worker's UPDATE thumbnail_path WHERE updated_at <=
	// started_at trips immediately. We re-read file.ThumbnailPath
	// after the UPDATE so we still clean up whatever was there
	// (including a thumbnail the worker may have just published
	// between our SELECT and this UPDATE).
	preUpdates := map[string]interface{}{"updated_at": time.Now().UTC(), "thumbnail_path": nil}
	if err := h.DB.Model(&models.File{}).Where("id = ?", file.ID).Updates(preUpdates).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating file metadata")
	}
	// Capture whatever thumbnail_path the row holds right now — could be
	// the value we read at SELECT, or a stale one the worker just wrote.
	// Either way the row is now nil, so any S3 object the path points
	// to is unreferenced and safe to delete.
	priorThumb := ""
	if file.ThumbnailPath != nil {
		priorThumb = *file.ThumbnailPath
	}

	if err := h.Storage.Upload(c.Context(), file.StoragePath, bytes.NewReader(body), int64(len(body)), file.MimeType); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed saving file content")
	}

	// Capture any thumbnail a viewer-enqueued worker may have published
	// in the gap between our pre-upload UPDATE (which cleared the path)
	// and now (after upload). Read this BEFORE the post-upload UPDATE,
	// since that UPDATE re-clears the path.
	var raceThumb sql.NullString
	_ = h.DB.Raw("SELECT thumbnail_path FROM files WHERE id = ?", file.ID).Scan(&raceThumb).Error

	// Update size AND clear thumbnail_path + bump updated_at again. A
	// viewer that enqueues a preview between our pre-upload UPDATE and
	// this one can land a stale render in that window: the new job's
	// started_at is after our first updated_at bump, so the fence
	// (file.updated_at <= started_at) passes and the worker writes
	// thumbnail_path against the pre-upload bytes. Re-clearing here
	// drops that stale pointer and re-fencing knocks out any further
	// in-flight worker that started before this final bump.
	postUpdates := map[string]interface{}{
		"size":           int64(len(body)),
		"updated_at":     time.Now().UTC(),
		"thumbnail_path": nil,
	}
	if err := h.DB.Model(&models.File{}).Where("id = ?", file.ID).Updates(postUpdates).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating file metadata")
	}
	if priorThumb != "" {
		if delErr := h.Storage.Delete(c.Context(), priorThumb); delErr != nil {
			logger.Error("preview_thumb_cleanup_failed", delErr, map[string]interface{}{
				"file_id":        file.ID.String(),
				"thumbnail_path": priorThumb,
			})
		}
	}
	if raceThumb.Valid && raceThumb.String != "" && raceThumb.String != priorThumb {
		// Worker raced in a stale thumbnail between our two updates. The
		// row no longer references it, so the S3 object is orphaned.
		if delErr := h.Storage.Delete(c.Context(), raceThumb.String); delErr != nil {
			logger.Error("preview_thumb_race_cleanup_failed", delErr, map[string]interface{}{
				"file_id":        file.ID.String(),
				"thumbnail_path": raceThumb.String,
			})
		}
	}
	if len(priorJobIDs) > 0 {
		// Delete only the jobs we snapshotted above. A replacement job
		// enqueued by the worker's superseded path (which sees our
		// updated_at bump) lives outside this id-set and survives.
		if err := h.DB.Where("id IN ?", priorJobIDs).Delete(&models.PreviewJob{}).Error; err != nil {
			logger.Error("preview_job_cleanup_failed", err, map[string]interface{}{
				"file_id": file.ID.String(),
			})
		}
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
