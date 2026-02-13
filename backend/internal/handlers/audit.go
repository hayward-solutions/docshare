package handlers

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type AuditHandler struct {
	DB *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{DB: db}
}

func (h *AuditHandler) ExportMyLog(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format", "csv")))
	if format != "csv" && format != "json" {
		return utils.Error(c, fiber.StatusBadRequest, "format must be csv or json")
	}

	var logs []models.AuditLog
	if err := h.DB.Where("user_id = ?", currentUser.ID).
		Order("created_at DESC").
		Limit(10000).
		Find(&logs).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading audit logs")
	}

	if format == "json" {
		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "audit-log.json"))
		return c.JSON(fiber.Map{"success": true, "data": logs})
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "audit-log.csv"))

	writer := csv.NewWriter(c.Response().BodyWriter())
	_ = writer.Write([]string{"Timestamp", "Action", "Resource Type", "Resource ID", "IP Address", "Details"})

	for _, log := range logs {
		resourceID := ""
		if log.ResourceID != nil {
			resourceID = log.ResourceID.String()
		}

		detailStr := ""
		if log.Details != nil {
			parts := make([]string, 0, len(log.Details))
			for k, v := range log.Details {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			detailStr = strings.Join(parts, "; ")
		}

		_ = writer.Write([]string{
			log.CreatedAt.Format(time.RFC3339),
			log.Action,
			log.ResourceType,
			resourceID,
			log.IPAddress,
			detailStr,
		})
	}

	writer.Flush()
	return nil
}
