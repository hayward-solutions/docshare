package handlers

import (
	"errors"
	"fmt"

	"github.com/docshare/api/internal/middleware"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/internal/services"
	"github.com/docshare/api/pkg/logger"
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Export converts a markdown or plain-text document to the requested
// format (pdf / docx / odt / rtf / html / epub / md / txt) and streams
// it back as an attachment. Gated on download permission so view-only
// shares can't pull the original bytes through this route.
func (h *FilesHandler) Export(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	if h.ExportService == nil {
		return utils.Error(c, fiber.StatusServiceUnavailable, "export service unavailable")
	}

	fileID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid file id")
	}

	format, ok := services.ParseFormat(c.Query("format"))
	if !ok {
		return utils.Error(c, fiber.StatusBadRequest, "unsupported export format")
	}

	var file models.File
	if err := h.DB.First(&file, "id = ?", fileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.Error(c, fiber.StatusNotFound, "file not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading file")
	}
	if file.IsDirectory {
		return utils.Error(c, fiber.StatusBadRequest, "cannot export a directory")
	}
	if !services.IsExportableSource(file.MimeType) {
		return utils.Error(c, fiber.StatusUnsupportedMediaType, "file type cannot be exported")
	}

	if !h.Access.HasAccess(c.Context(), currentUser.ID, file.ID, models.SharePermissionDownload) {
		logger.WarnWithUser(currentUser.ID.String(), "permission_denied", map[string]interface{}{
			"action":              "file_export",
			"target_id":           file.ID.String(),
			"file_name":           file.Name,
			"required_permission": "download",
		})
		return utils.Error(c, fiber.StatusForbidden, "access denied")
	}

	result, err := h.ExportService.Export(c.Context(), &file, format)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrFormatNotSupported):
			return utils.Error(c, fiber.StatusBadRequest, "format not supported for this file type")
		case errors.Is(err, services.ErrPandocMissing):
			return utils.Error(c, fiber.StatusServiceUnavailable, "this format requires pandoc, which is not installed on the server")
		default:
			logger.Error("file_export_failed", err, map[string]interface{}{
				"file_id": file.ID.String(),
				"format":  string(format),
			})
			return utils.Error(c, fiber.StatusInternalServerError, "export failed")
		}
	}

	logger.InfoWithUser(currentUser.ID.String(), "file_exported", map[string]interface{}{
		"file_id":     file.ID.String(),
		"file_name":   file.Name,
		"format":      string(format),
		"output_size": len(result.Body),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "file.export",
		ResourceType: "file",
		ResourceID:   &file.ID,
		Details: map[string]interface{}{
			"file_name": file.Name,
			"format":    string(format),
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	c.Set("Content-Type", result.MimeType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.Filename))
	return c.Send(result.Body)
}
