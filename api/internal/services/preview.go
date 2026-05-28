package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/internal/storage"
	"github.com/docshare/api/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PreviewService struct {
	DB         *gorm.DB
	Storage    *storage.S3Client
	Gotenberg  config.GotenbergConfig
	HTTPClient *http.Client
}

func NewPreviewService(db *gorm.DB, storageClient *storage.S3Client, gotenberg config.GotenbergConfig) *PreviewService {
	return &PreviewService{
		DB:        db,
		Storage:   storageClient,
		Gotenberg: gotenberg,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ErrPreviewSuperseded is returned by ConvertToPreview when the file was
// edited after the conversion started, so the freshly-rendered PDF is
// already stale. Callers (the preview queue) should re-enqueue against
// the latest bytes rather than marking the job completed-without-thumbnail.
var ErrPreviewSuperseded = errors.New("preview superseded by later file edit")

// ConvertToPreview renders a PDF preview for file and publishes the
// thumbnail_path so the viewer serves it. The notAfter parameter guards
// against a race: if the file was edited after the caller decided to run
// this conversion (e.g. SaveBinary cleared the previous thumbnail mid-
// render), the publish becomes a no-op, the freshly-uploaded PDF is
// cleaned up, and ErrPreviewSuperseded is returned so the queue can
// re-enqueue against the current bytes.
// Callers pass the job's StartedAt as notAfter; a zero value skips the
// guard.
func (p *PreviewService) ConvertToPreview(ctx context.Context, file *models.File, notAfter time.Time) (string, error) {
	if file.IsDirectory {
		return "", fmt.Errorf("cannot preview a directory")
	}

	if !isOfficeDocument(file.Name) {
		return p.Storage.PresignedGetURLWithResponse(ctx, file.StoragePath, 15*time.Minute, file.MimeType, "inline")
	}

	sourceObject, err := p.Storage.Download(ctx, file.StoragePath)
	if err != nil {
		return "", err
	}
	defer sourceObject.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		part, partErr := writer.CreateFormFile("files", file.Name)
		if partErr != nil {
			_ = pw.CloseWithError(partErr)
			return
		}

		if _, copyErr := io.Copy(part, sourceObject); copyErr != nil {
			_ = pw.CloseWithError(copyErr)
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.Gotenberg.URL, "/")+"/forms/libreoffice/convert", pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("gotenberg conversion failed: %s", string(body))
	}

	previewPath := fmt.Sprintf("%s/previews/%s.pdf", file.OwnerID.String(), uuid.New().String())
	if err := p.Storage.Upload(ctx, previewPath, resp.Body, -1, "application/pdf"); err != nil {
		return "", err
	}

	// Gate the publish on the file not having been edited since the job
	// started. If a SaveBinary landed mid-render, file.updated_at has
	// advanced past notAfter and the UPDATE matches 0 rows — we then
	// drop the thumbnail bytes we just uploaded and bail without
	// claiming a stale PDF as the current preview.
	q := p.DB.WithContext(ctx).Model(&models.File{}).Where("id = ?", file.ID)
	if !notAfter.IsZero() {
		q = q.Where("updated_at <= ?", notAfter)
	}
	result := q.Update("thumbnail_path", previewPath)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		// File was edited mid-render; the bytes we just rendered are stale.
		// Best-effort cleanup so we don't leak orphan PDFs in S3, then
		// signal the queue to re-enqueue against the latest bytes.
		if delErr := p.Storage.Delete(ctx, previewPath); delErr != nil {
			logger.Error("preview_stale_thumb_cleanup_failed", delErr, map[string]interface{}{
				"file_id":      file.ID.String(),
				"preview_path": previewPath,
			})
		}
		logger.Info("preview_publish_skipped_stale", map[string]interface{}{
			"file_id": file.ID.String(),
		})
		return "", ErrPreviewSuperseded
	}
	file.ThumbnailPath = &previewPath

	return p.Storage.PresignedGetURLWithResponse(ctx, previewPath, 15*time.Minute, "application/pdf", "inline")
}

func isOfficeDocument(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp":
		return true
	default:
		return false
	}
}
