package handlers

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docshare/api/internal/middleware"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/internal/services"
	"github.com/docshare/api/internal/storage"
	"github.com/docshare/api/pkg/logger"
	"github.com/docshare/api/pkg/previewtoken"
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

const presignedUploadTTL = 30 * time.Minute

// uploadStagingPrefix scopes keys handed out by the presign endpoint. After
// finalize, the object is moved out of this prefix (to its final
// storage_path), so any later writes through the still-valid presigned URL
// land at a key that nothing references — isolating finalized files from
// post-finalize overwrites.
const uploadStagingPrefix = "uploads/"

// s3SinglePutMaxBytes is AWS S3's hard ceiling for a single PUT request
// (5 GiB). The presign flow issues a single-PUT URL, so we cap there
// regardless of MAX_UPLOAD_MB — otherwise the client would happily try to
// PUT 10 GB and only learn at upload time that S3 refuses. Going above
// requires switching to multipart presigned uploads.
const s3SinglePutMaxBytes int64 = 5 * 1024 * 1024 * 1024

type FilesHandler struct {
	DB             *gorm.DB
	Storage        *storage.S3Client
	Access         *services.AccessService
	PreviewService *services.PreviewService
	PreviewQueue   *services.PreviewQueueService
	ExportService  *services.ExportService
	Audit          *services.AuditService
	MaxUploadBytes int64
}

func NewFilesHandler(db *gorm.DB, storageClient *storage.S3Client, access *services.AccessService, preview *services.PreviewService, previewQueue *services.PreviewQueueService, export *services.ExportService, audit *services.AuditService, maxUploadBytes int64) *FilesHandler {
	return &FilesHandler{DB: db, Storage: storageClient, Access: access, PreviewService: preview, PreviewQueue: previewQueue, ExportService: export, Audit: audit, MaxUploadBytes: maxUploadBytes}
}

// maybeEnqueueImageThumbnail fires the preview pipeline for image uploads so
// the grid can render a small JPEG thumbnail without pulling the original.
// Enqueue is best-effort: a queue-full or dedup hit must not fail the upload
// itself. PreviewQueue.Enqueue already deduplicates by file_id, so racing
// callers (e.g. multi-tab uploads) won't double-up.
func (h *FilesHandler) maybeEnqueueImageThumbnail(file *models.File, requestedBy *uuid.UUID) {
	if file == nil || file.IsDirectory {
		return
	}
	if h.PreviewQueue == nil {
		return
	}
	if !services.IsThumbnailableImage(file.MimeType) {
		return
	}
	if _, err := h.PreviewQueue.Enqueue(file.ID, requestedBy); err != nil {
		logger.Error("image_thumbnail_enqueue_failed", err, map[string]interface{}{
			"file_id":   file.ID.String(),
			"mime_type": file.MimeType,
		})
	}
}

func resolveMimeType(filename, declared string) string {
	contentType := declared
	// "" and application/octet-stream are both "the caller didn't say" —
	// the multipart upload path used by the CLI sends octet-stream as a
	// default because Go's mime/multipart doesn't sniff. Prefer the
	// extension when it yields something specific, otherwise keep the
	// generic fallback. Without this, CLI-uploaded .jpg/.png/.pdf files
	// landed as application/octet-stream and downstream gates (image
	// thumbnail enqueue, viewer routing) silently skipped them.
	if contentType == "" || contentType == "application/octet-stream" {
		if ext := mime.TypeByExtension(filepath.Ext(filename)); ext != "" {
			contentType = ext
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md", ".markdown":
		contentType = "text/markdown"
	case ".ts":
		contentType = "text/typescript"
	case ".tsx":
		contentType = "text/tsx"
	}
	return contentType
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

	contentType := resolveMimeType(filename, fileHeader.Header.Get("Content-Type"))

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

	h.maybeEnqueueImageThumbnail(&entry, &currentUser.ID)

	return utils.Success(c, fiber.StatusCreated, entry)
}

type presignUploadRequest struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	MimeType string  `json:"mimeType"`
	ParentID *string `json:"parentID"`
}

type presignUploadResponse struct {
	UploadURL string    `json:"uploadURL"`
	Key       string    `json:"key"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (h *FilesHandler) PresignUpload(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req presignUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	filename := filepath.Base(strings.TrimSpace(req.Name))
	if filename == "" || filename == "." || filename == "/" {
		return utils.Error(c, fiber.StatusBadRequest, "invalid filename")
	}

	if req.Size <= 0 {
		return utils.Error(c, fiber.StatusBadRequest, "size must be positive")
	}
	if h.MaxUploadBytes > 0 && req.Size > h.MaxUploadBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds maximum upload size of %d bytes", h.MaxUploadBytes))
	}
	if req.Size > s3SinglePutMaxBytes {
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds 5 GiB single-PUT limit for pre-signed uploads (got %d bytes)", req.Size))
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
			logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
				"action":              "file_upload",
				"target_id":           parent.ID.String(),
				"required_permission": "edit",
			})
			return utils.Error(c, fiber.StatusForbidden, "no permission to upload to parent directory")
		}
		parentID = &parent.ID
	}

	// Issue the URL against the staging prefix; finalize will copy the object
	// out of staging so this URL no longer addresses live content. Sign
	// Content-Length into the URL so the holder can only PUT exactly the
	// number of bytes they claimed — without this, a client could presign
	// for 1 KB and upload 100 GB to staging.
	objectName := fmt.Sprintf("%s%s/%s/%s", uploadStagingPrefix, currentUser.ID.String(), uuid.New().String(), filename)
	uploadURL, presignErr := h.Storage.PresignedPutURLWithLength(c.Context(), objectName, presignedUploadTTL, req.Size)
	if presignErr != nil {
		logger.Error("s3_presign_put_failed", presignErr, map[string]interface{}{
			"object_name": objectName,
			"user_id":     currentUser.ID.String(),
		})
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating upload URL")
	}

	logger.InfoWithUser(currentUser.ID.String(), "upload_presigned", map[string]interface{}{
		"object_name": objectName,
		"file_name":   filename,
		"size":        req.Size,
		"parent_id":   parentID,
	})

	return utils.Success(c, fiber.StatusOK, presignUploadResponse{
		UploadURL: uploadURL,
		Key:       objectName,
		ExpiresAt: time.Now().Add(presignedUploadTTL),
	})
}

type finalizeUploadRequest struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	MimeType string  `json:"mimeType"`
	ParentID *string `json:"parentID"`
}

func (h *FilesHandler) FinalizeUpload(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req finalizeUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	rawKey := strings.TrimSpace(req.Key)
	if rawKey == "" {
		return utils.Error(c, fiber.StatusBadRequest, "key is required")
	}
	// path.Clean resolves any "../" segments so the prefix check below cannot
	// be bypassed (e.g. "uploads/uA/../uB/...") and gives us a canonical key.
	stagingKey := path.Clean(rawKey)
	expectedStagingPrefix := uploadStagingPrefix + currentUser.ID.String() + "/"
	if !strings.HasPrefix(stagingKey, expectedStagingPrefix) {
		logger.WarnWithUser(currentUser.ID.String(), "upload_finalize_foreign_key", map[string]interface{}{
			"key":     rawKey,
			"cleaned": stagingKey,
		})
		return utils.Error(c, fiber.StatusForbidden, "key does not belong to authenticated user")
	}
	// finalKey is what we persist as storage_path. Stripping the staging
	// prefix gives `{userID}/{uuid}/{filename}` — the same shape used by the
	// legacy multipart upload path, so downstream code (downloads, previews,
	// audit) doesn't need to change.
	finalKey := strings.TrimPrefix(stagingKey, uploadStagingPrefix)

	filename := filepath.Base(strings.TrimSpace(req.Name))
	if filename == "" || filename == "." || filename == "/" {
		return utils.Error(c, fiber.StatusBadRequest, "invalid filename")
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
			logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
				"action":              "file_upload",
				"target_id":           parent.ID.String(),
				"required_permission": "edit",
			})
			return utils.Error(c, fiber.StatusForbidden, "no permission to upload to parent directory")
		}
		parentID = &parent.ID
	}

	// Cheap fast-path for sequential replays: skip the S3 stat/copy work
	// entirely if a (non-soft-deleted) row already references finalKey. The
	// transactional Create below remains the source of truth for concurrent
	// racers.
	var existing int64
	if err := h.DB.Model(&models.File{}).Where("storage_path = ?", finalKey).Count(&existing).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed checking file existence")
	}
	if existing > 0 {
		return utils.Error(c, fiber.StatusConflict, "upload already finalized")
	}

	info, statErr := h.Storage.StatObject(c.Context(), stagingKey)
	if statErr != nil {
		errResp := minio.ToErrorResponse(statErr)
		if errResp.Code == "NoSuchKey" || errResp.StatusCode == fiber.StatusNotFound {
			return utils.Error(c, fiber.StatusNotFound, "uploaded object not found")
		}
		logger.Error("s3_stat_failed", statErr, map[string]interface{}{
			"object_name": stagingKey,
			"user_id":     currentUser.ID.String(),
		})
		return utils.Error(c, fiber.StatusInternalServerError, "failed verifying uploaded object")
	}

	if h.MaxUploadBytes > 0 && info.Size > h.MaxUploadBytes {
		_ = h.Storage.Delete(c.Context(), stagingKey)
		return utils.Error(c, fiber.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds maximum upload size of %d bytes", h.MaxUploadBytes))
	}

	contentType := resolveMimeType(filename, req.MimeType)

	entry := models.File{
		Name:        filename,
		MimeType:    contentType,
		Size:        info.Size,
		IsDirectory: false,
		ParentID:    parentID,
		OwnerID:     currentUser.ID,
		StoragePath: finalKey,
	}

	// Claim → copy → commit. Inserting the row first inside a transaction
	// turns the storage_path unique index into a race-safe gate: a concurrent
	// finalize attempting to insert the same finalKey blocks on the index
	// until we commit (then fails with duplicate-key) or rolls back (then
	// succeeds). This stops the loser from ever calling CopyObject — without
	// this, both finalizers would race-copy and the second copy could write
	// different bytes if the staging URL was used to overwrite between
	// stats. The MatchETag pinned on the copy further guarantees that the
	// bytes we land at finalKey are the ones we stat'd, even if the staging
	// key is overwritten between stat and copy.
	txErr := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return h.Storage.CopyObject(c.Context(), finalKey, stagingKey, info.ETag)
	})
	if txErr != nil {
		if errors.Is(txErr, gorm.ErrDuplicatedKey) {
			return utils.Error(c, fiber.StatusConflict, "upload already finalized")
		}
		// Either the copy failed or another DB error fired. The transaction
		// rolled back the row, but the copy may have partially written to
		// finalKey before failing — best-effort clean it up. Staging is left
		// alone so the caller can retry finalize without re-uploading.
		_ = h.Storage.Delete(c.Context(), finalKey)
		return utils.Error(c, fiber.StatusInternalServerError, "failed promoting upload")
	}

	// Best-effort cleanup of the staging object. If this fails the file row
	// is already pointing at finalKey, so the failure only leaves an orphan
	// in staging — addressable later via a lifecycle rule or sweeper.
	if err := h.Storage.Delete(c.Context(), stagingKey); err != nil {
		logger.Error("s3_staging_cleanup_failed", err, map[string]interface{}{
			"staging_key": stagingKey,
			"final_key":   finalKey,
			"file_id":     entry.ID.String(),
		})
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_uploaded", map[string]interface{}{
		"file_id":      entry.ID.String(),
		"file_name":    filename,
		"file_size":    info.Size,
		"mime_type":    contentType,
		"storage_path": finalKey,
		"staging_key":  stagingKey,
		"parent_id":    parentID,
		"upload_mode":  "presigned",
	})

	auditDetails := map[string]interface{}{
		"file_name":   filename,
		"file_size":   info.Size,
		"mime_type":   contentType,
		"upload_mode": "presigned",
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

	h.maybeEnqueueImageThumbnail(&entry, &currentUser.ID)

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

	p := utils.ParsePagination(c)
	sort := utils.ParseFileSort(c)

	var owned []models.File
	if err := h.DB.Preload("Owner").Where("owner_id = ? AND parent_id IS NULL", currentUser.ID).Find(&owned).Error; err != nil {
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

	sort.SortFiles(combined)
	total := int64(len(combined))

	start := p.Offset
	end := start + p.Limit
	if start > len(combined) {
		combined = []models.File{}
	} else {
		if end > len(combined) {
			end = len(combined)
		}
		combined = combined[start:end]
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

	return utils.Paginated(c, combined, p.Page, p.Limit, total)
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

	// Populate the transient permission flags so the file viewer can hide
	// the Edit button when the user lacks the byte-level access the
	// editor's /binary or /content fetch will require.
	isOwner := file.OwnerID == currentUser.ID
	file.CanEdit = isOwner || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionEdit)
	file.CanDownload = file.CanEdit || h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionDownload)

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

	p := utils.ParsePagination(c)

	var total int64
	if err := h.DB.Model(&models.File{}).Where("parent_id = ?", parent.ID).Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting children")
	}

	var children []models.File
	query := h.DB.Preload("Owner").Where("parent_id = ?", parent.ID).Order(utils.ParseFileSort(c).SQLClause())
	if err := utils.ApplyPagination(query, p).Find(&children).Error; err != nil {
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

	return utils.Paginated(c, children, p.Page, p.Limit, total)
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

	// Prefer the MIME type stored in our DB: pre-signed PUT uploads land in S3
	// as application/octet-stream (we don't sign Content-Type into the PUT URL),
	// so S3's reported content-type isn't authoritative.
	contentType := file.MimeType
	if contentType == "" {
		contentType = stat.ContentType
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

	// The variant param is propagated into the returned path so the client
	// builds one URL: ?variant=thumb selects the small JPEG thumbnail (for
	// grid view); absent/any other value selects the renderable form (for
	// the viewer: original for images, PDF render for Office docs).
	path := "/files/" + fileID.String() + "/proxy"
	if c.Query("variant") == "thumb" {
		path += "?variant=thumb"
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"path":  path,
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

	// Path selection:
	//   variant=thumb  → force the small derived asset (ThumbnailPath);
	//                    404 if none exists so the grid can fall back to
	//                    the icon without downloading the full original.
	//   default         → the renderable form. For images, that's the
	//                    full StoragePath (the viewer needs the original
	//                    resolution; the 400px JPEG would render blurry).
	//                    For non-images with a generated preview (e.g.
	//                    Office → PDF), that's still ThumbnailPath.
	variant := c.Query("variant")
	isImage := strings.HasPrefix(file.MimeType, "image/")
	hasThumbnail := file.ThumbnailPath != nil && *file.ThumbnailPath != ""

	storagePath := file.StoragePath
	servingThumbnail := false
	if variant == "thumb" {
		if !hasThumbnail {
			return utils.Error(c, fiber.StatusNotFound, "thumbnail not available")
		}
		storagePath = *file.ThumbnailPath
		servingThumbnail = true
	} else if hasThumbnail && !isImage {
		storagePath = *file.ThumbnailPath
		servingThumbnail = true
	}

	obj, err := h.Storage.Download(c.Context(), storagePath)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed downloading file")
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return utils.Error(c, fiber.StatusInternalServerError, "failed reading object metadata")
	}

	// When we're serving a derived thumbnail, S3's reported content-type is
	// authoritative (PreviewService uploaded with the correct one — JPEG
	// for image thumbnails, PDF for Office-doc thumbnails). For the
	// original, prefer DB MimeType, since pre-signed PUT uploads land in
	// S3 as application/octet-stream.
	var contentType string
	if servingThumbnail {
		contentType = stat.ContentType
		if contentType == "" {
			if isImage {
				contentType = "image/jpeg"
			} else {
				contentType = "application/pdf"
			}
		}
	} else {
		contentType = file.MimeType
		if contentType == "" {
			contentType = stat.ContentType
		}
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

	job, err := h.PreviewQueue.Enqueue(file.ID, &currentUser.ID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to enqueue preview job")
	}

	return utils.Success(c, fiber.StatusAccepted, fiber.Map{
		"job": h.jobToResponse(job, &file),
	})
}

func (h *FilesHandler) PreviewStatus(c *fiber.Ctx) error {
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

	job, err := h.PreviewQueue.GetJobByFileID(fileID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to get preview status")
	}

	if job == nil {
		return utils.Success(c, fiber.StatusOK, fiber.Map{
			"job":  nil,
			"file": file,
		})
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"job":  h.jobToResponse(job, &file),
		"file": file,
	})
}

func (h *FilesHandler) RetryPreview(c *fiber.Ctx) error {
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

	job, err := h.PreviewQueue.Retry(fileID, &currentUser.ID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to retry preview job")
	}

	return utils.Success(c, fiber.StatusAccepted, fiber.Map{
		"job": h.jobToResponse(job, &file),
	})
}

func (h *FilesHandler) jobToResponse(job *models.PreviewJob, file *models.File) fiber.Map {
	response := fiber.Map{
		"id":          job.ID,
		"fileID":      job.FileID,
		"status":      job.Status,
		"attempts":    job.Attempts,
		"maxAttempts": job.MaxAttempts,
		"createdAt":   job.CreatedAt,
		"updatedAt":   job.UpdatedAt,
	}

	if job.LastError != nil {
		response["lastError"] = *job.LastError
	}
	if job.StartedAt != nil {
		response["startedAt"] = *job.StartedAt
	}
	if job.CompletedAt != nil {
		response["completedAt"] = *job.CompletedAt
	}
	if job.NextRetryAt != nil {
		response["nextRetryAt"] = *job.NextRetryAt
	}
	if file != nil && file.ThumbnailPath != nil {
		response["thumbnailPath"] = *file.ThumbnailPath
	}

	return response
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

	p := utils.ParsePagination(c)
	searchValue := "%" + strings.ToLower(q) + "%"
	directoryIDRaw := strings.TrimSpace(c.Query("directoryID"))

	var files []models.File
	var total int64

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
			return utils.Paginated(c, []models.File{}, p.Page, p.Limit, 0)
		}

		ids := make([]uuid.UUID, len(descendantIDs))
		for i, d := range descendantIDs {
			ids[i] = d.ID
		}

		countQuery := h.DB.Model(&models.File{}).Where("id IN ? AND LOWER(name) LIKE ?", ids, searchValue)
		if err := countQuery.Count(&total).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}

		if err := h.DB.Preload("Owner").
			Where("id IN ? AND LOWER(name) LIKE ?", ids, searchValue).
			Order(utils.ParseFileSort(c).SQLClause()).
			Offset(p.Offset).
			Limit(p.Limit).
			Find(&files).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}
	} else {
		countQuery := h.DB.Model(&models.File{}).Where("owner_id = ? AND LOWER(name) LIKE ?", currentUser.ID, searchValue)
		if err := countQuery.Count(&total).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}

		if err := h.DB.Preload("Owner").
			Where("owner_id = ? AND LOWER(name) LIKE ?", currentUser.ID, searchValue).
			Order(utils.ParseFileSort(c).SQLClause()).
			Offset(p.Offset).
			Limit(p.Limit).
			Find(&files).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "search failed")
		}
	}

	h.enrichParentNames(files)

	return utils.Paginated(c, files, p.Page, p.Limit, total)
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

	p := utils.ParsePagination(c)

	var total int64
	if err := h.DB.Model(&models.File{}).Where("parent_id = ?", parent.ID).Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting children")
	}

	var children []models.File
	query := h.DB.Preload("Owner").Where("parent_id = ?", parent.ID).Order(utils.ParseFileSort(c).SQLClause())
	if err := utils.ApplyPagination(query, p).Find(&children).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading children")
	}

	return utils.Paginated(c, children, p.Page, p.Limit, total)
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

	// Prefer the MIME type stored in our DB: pre-signed PUT uploads land in S3
	// as application/octet-stream (we don't sign Content-Type into the PUT URL),
	// so S3's reported content-type isn't authoritative.
	contentType := file.MimeType
	if contentType == "" {
		contentType = stat.ContentType
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
