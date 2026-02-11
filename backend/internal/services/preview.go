package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PreviewService struct {
	DB         *gorm.DB
	Storage    *storage.MinIOClient
	Gotenberg  config.GotenbergConfig
	HTTPClient *http.Client
}

func NewPreviewService(db *gorm.DB, storageClient *storage.MinIOClient, gotenberg config.GotenbergConfig) *PreviewService {
	return &PreviewService{
		DB:        db,
		Storage:   storageClient,
		Gotenberg: gotenberg,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *PreviewService) ConvertToPreview(ctx context.Context, file *models.File) (string, error) {
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

	file.ThumbnailPath = &previewPath
	if err := p.DB.WithContext(ctx).Model(&models.File{}).Where("id = ?", file.ID).Update("thumbnail_path", previewPath).Error; err != nil {
		return "", err
	}

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
