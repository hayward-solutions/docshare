package handlers

import (
	"strings"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UsersHandler struct {
	DB *gorm.DB
}

func NewUsersHandler(db *gorm.DB) *UsersHandler {
	return &UsersHandler{DB: db}
}

func (h *UsersHandler) List(c *fiber.Ctx) error {
	p := utils.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))

	query := h.DB.Model(&models.User{})
	if search != "" {
		searchValue := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"LOWER(email) LIKE ? OR LOWER(first_name) LIKE ? OR LOWER(last_name) LIKE ?",
			searchValue,
			searchValue,
			searchValue,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting users")
	}

	var users []models.User
	if err := utils.ApplyPagination(query.Order("created_at DESC"), p).Find(&users).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed listing users")
	}

	return utils.Paginated(c, users, p.Page, p.Limit, total)
}

func (h *UsersHandler) Search(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	search := strings.TrimSpace(c.Query("search"))
	limit := c.QueryInt("limit", 5)

	if limit > 50 {
		limit = 50
	}

	if search != "" && currentUser != nil {
		logger.InfoWithUser(currentUser.ID.String(), "user_search", map[string]interface{}{
			"query": search,
			"limit": limit,
		})
	}

	query := h.DB.Model(&models.User{})
	if search != "" {
		searchValue := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"LOWER(email) LIKE ? OR LOWER(first_name) LIKE ? OR LOWER(last_name) LIKE ?",
			searchValue,
			searchValue,
			searchValue,
		)
	}

	var users []models.User
	if err := query.Order("created_at DESC").Limit(limit).Find(&users).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed searching users")
	}

	return utils.Success(c, fiber.StatusOK, users)
}

func (h *UsersHandler) Get(c *fiber.Ctx) error {
	userID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid user id")
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "user not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed fetching user")
	}

	return utils.Success(c, fiber.StatusOK, user)
}

type updateUserRequest struct {
	FirstName *string          `json:"firstName"`
	LastName  *string          `json:"lastName"`
	AvatarURL *string          `json:"avatarURL"`
	Role      *models.UserRole `json:"role"`
}

func (h *UsersHandler) Update(c *fiber.Ctx) error {
	userID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid user id")
	}

	var req updateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	updates := map[string]interface{}{}
	if req.FirstName != nil {
		value := strings.TrimSpace(*req.FirstName)
		if value == "" {
			return utils.Error(c, fiber.StatusBadRequest, "firstName cannot be empty")
		}
		updates["first_name"] = value
	}
	if req.LastName != nil {
		value := strings.TrimSpace(*req.LastName)
		if value == "" {
			return utils.Error(c, fiber.StatusBadRequest, "lastName cannot be empty")
		}
		updates["last_name"] = value
	}
	if req.AvatarURL != nil {
		trimmed := strings.TrimSpace(*req.AvatarURL)
		if trimmed == "" {
			updates["avatar_url"] = nil
		} else {
			updates["avatar_url"] = trimmed
		}
	}
	if req.Role != nil {
		if *req.Role != models.UserRoleAdmin && *req.Role != models.UserRoleUser {
			return utils.Error(c, fiber.StatusBadRequest, "invalid role")
		}
		updates["role"] = *req.Role
	}

	if len(updates) == 0 {
		return utils.Error(c, fiber.StatusBadRequest, "no valid fields to update")
	}

	result := h.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
	if result.Error != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating user")
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, fiber.StatusNotFound, "user not found")
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed fetching updated user")
	}

	return utils.Success(c, fiber.StatusOK, user)
}

func (h *UsersHandler) Delete(c *fiber.Ctx) error {
	userID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid user id")
	}

	result := h.DB.Delete(&models.User{}, "id = ?", userID)
	if result.Error != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed deleting user")
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, fiber.StatusNotFound, "user not found")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "user deleted"})
}
