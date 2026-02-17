package handlers

import (
	"github.com/docshare/api/internal/middleware"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ActivitiesHandler struct {
	DB *gorm.DB
}

func NewActivitiesHandler(db *gorm.DB) *ActivitiesHandler {
	return &ActivitiesHandler{DB: db}
}

func (h *ActivitiesHandler) List(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	p := utils.ParsePagination(c)

	query := h.DB.Model(&models.Activity{}).Where("user_id = ?", currentUser.ID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting activities")
	}

	var activities []models.Activity
	if err := utils.ApplyPagination(
		h.DB.Preload("Actor").Where("user_id = ?", currentUser.ID).Order("created_at DESC"),
		p,
	).Find(&activities).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed listing activities")
	}

	return utils.Paginated(c, activities, p.Page, p.Limit, total)
}

func (h *ActivitiesHandler) UnreadCount(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var count int64
	if err := h.DB.Model(&models.Activity{}).
		Where("user_id = ? AND is_read = false", currentUser.ID).
		Count(&count).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting unread activities")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"count": count})
}

func (h *ActivitiesHandler) MarkRead(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	activityID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid activity id")
	}

	result := h.DB.Model(&models.Activity{}).
		Where("id = ? AND user_id = ?", activityID, currentUser.ID).
		Update("is_read", true)

	if result.Error != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed marking activity as read")
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, fiber.StatusNotFound, "activity not found")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "marked as read"})
}

func (h *ActivitiesHandler) MarkAllRead(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	if err := h.DB.Model(&models.Activity{}).
		Where("user_id = ? AND is_read = false", currentUser.ID).
		Update("is_read", true).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed marking all activities as read")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "all marked as read"})
}
