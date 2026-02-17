package handlers

import (
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

type GroupsHandler struct {
	DB    *gorm.DB
	Audit *services.AuditService
}

func NewGroupsHandler(db *gorm.DB, audit *services.AuditService) *GroupsHandler {
	return &GroupsHandler{DB: db, Audit: audit}
}

type createGroupRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

func (h *GroupsHandler) Create(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req createGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return utils.Error(c, fiber.StatusBadRequest, "name is required")
	}

	group := models.Group{
		Name:        req.Name,
		Description: req.Description,
		CreatedByID: currentUser.ID,
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&group).Error; err != nil {
			return err
		}

		membership := models.GroupMembership{
			UserID:  currentUser.ID,
			GroupID: group.ID,
			Role:    models.GroupRoleOwner,
		}
		return tx.Create(&membership).Error
	})
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating group")
	}

	logger.InfoWithUser(currentUser.ID.String(), "group_created", map[string]interface{}{
		"group_id":   group.ID.String(),
		"group_name": group.Name,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "group.create",
		ResourceType: "group",
		ResourceID:   &group.ID,
		Details: map[string]interface{}{
			"group_name": group.Name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, group)
}

func (h *GroupsHandler) List(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	p := utils.ParsePagination(c)

	baseQuery := h.DB.
		Model(&models.Group{}).
		Preload("Memberships").
		Joins("JOIN group_memberships ON group_memberships.group_id = groups.id").
		Where("group_memberships.user_id = ?", currentUser.ID)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed counting groups")
	}

	var groups []models.Group
	if err := utils.ApplyPagination(baseQuery.Order("groups.created_at DESC"), p).Find(&groups).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed listing groups")
	}

	return utils.Paginated(c, groups, p.Page, p.Limit, total)
}

func (h *GroupsHandler) Get(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	groupID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid group id")
	}

	if _, err := h.getMembership(groupID, currentUser.ID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusForbidden, "group access denied")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed validating membership")
	}

	var group models.Group
	if err := h.DB.Preload("Memberships.User").First(&group, "id = ?", groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "group not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading group")
	}

	return utils.Success(c, fiber.StatusOK, group)
}

type updateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *GroupsHandler) Update(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	groupID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid group id")
	}

	membership, err := h.getMembership(groupID, currentUser.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusForbidden, "group access denied")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed validating membership")
	}
	if membership.Role != models.GroupRoleOwner && membership.Role != models.GroupRoleAdmin {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	var req updateGroupRequest
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
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if trimmed == "" {
			updates["description"] = nil
		} else {
			updates["description"] = trimmed
		}
	}

	if len(updates) == 0 {
		return utils.Error(c, fiber.StatusBadRequest, "no valid fields to update")
	}

	result := h.DB.Model(&models.Group{}).Where("id = ?", groupID).Updates(updates)
	if result.Error != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating group")
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, fiber.StatusNotFound, "group not found")
	}

	var updated models.Group
	if err := h.DB.First(&updated, "id = ?", groupID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading updated group")
	}

	return utils.Success(c, fiber.StatusOK, updated)
}

func (h *GroupsHandler) Delete(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	groupID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid group id")
	}

	membership, err := h.getMembership(groupID, currentUser.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusForbidden, "group access denied")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed validating membership")
	}
	if membership.Role != models.GroupRoleOwner {
		return utils.Error(c, fiber.StatusForbidden, "only group owner can delete the group")
	}

	var group models.Group
	h.DB.Select("id", "name").First(&group, "id = ?", groupID)

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&models.GroupMembership{}).Error; err != nil {
			return err
		}
		if err := tx.Where("shared_with_group_id = ?", groupID).Delete(&models.Share{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Group{}, "id = ?", groupID).Error
	})
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed deleting group")
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "group.delete",
		ResourceType: "group",
		ResourceID:   &groupID,
		Details: map[string]interface{}{
			"group_name": group.Name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "group deleted"})
}

type addMemberRequest struct {
	UserID uuid.UUID                  `json:"userID"`
	Role   models.GroupMembershipRole `json:"role"`
}

func (h *GroupsHandler) AddMember(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	groupID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid group id")
	}

	actorMembership, err := h.getMembership(groupID, currentUser.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusForbidden, "group access denied")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed validating membership")
	}
	if actorMembership.Role != models.GroupRoleOwner && actorMembership.Role != models.GroupRoleAdmin {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	var req addMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.UserID == uuid.Nil {
		return utils.Error(c, fiber.StatusBadRequest, "userID is required")
	}
	if req.Role != models.GroupRoleOwner && req.Role != models.GroupRoleAdmin && req.Role != models.GroupRoleMember {
		return utils.Error(c, fiber.StatusBadRequest, "invalid role")
	}
	if actorMembership.Role == models.GroupRoleAdmin && req.Role != models.GroupRoleMember {
		return utils.Error(c, fiber.StatusForbidden, "admins can only add members with member role")
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", req.UserID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "user not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading user")
	}

	membership := models.GroupMembership{
		UserID:  req.UserID,
		GroupID: groupID,
		Role:    req.Role,
	}

	if err := h.DB.Create(&membership).Error; err != nil {
		return utils.Error(c, fiber.StatusConflict, "user is already a member")
	}

	var grp models.Group
	h.DB.Select("name").First(&grp, "id = ?", groupID)

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "group.member_add",
		ResourceType: "group",
		ResourceID:   &groupID,
		Details: map[string]interface{}{
			"target_user_id": req.UserID.String(),
			"role":           string(req.Role),
			"group_name":     grp.Name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, membership)
}

func (h *GroupsHandler) RemoveMember(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	groupID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid group id")
	}
	userID, err := parseUUID(c.Params("userId"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid user id")
	}

	actorMembership, err := h.getMembership(groupID, currentUser.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusForbidden, "group access denied")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed validating membership")
	}

	targetMembership, err := h.getMembership(groupID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "member not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading target membership")
	}

	if targetMembership.Role == models.GroupRoleOwner {
		return utils.Error(c, fiber.StatusForbidden, "cannot remove group owner")
	}
	if actorMembership.Role != models.GroupRoleOwner && actorMembership.Role != models.GroupRoleAdmin {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}
	if actorMembership.Role == models.GroupRoleAdmin && targetMembership.Role == models.GroupRoleAdmin {
		return utils.Error(c, fiber.StatusForbidden, "admins cannot remove other admins")
	}

	if err := h.DB.Delete(&models.GroupMembership{}, "id = ?", targetMembership.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed removing member")
	}

	var grp models.Group
	h.DB.Select("name").First(&grp, "id = ?", groupID)

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "group.member_remove",
		ResourceType: "group",
		ResourceID:   &groupID,
		Details: map[string]interface{}{
			"target_user_id": userID.String(),
			"group_name":     grp.Name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "member removed"})
}

type updateMemberRoleRequest struct {
	Role models.GroupMembershipRole `json:"role"`
}

func (h *GroupsHandler) UpdateMemberRole(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	groupID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid group id")
	}
	userID, err := parseUUID(c.Params("userId"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid user id")
	}

	actorMembership, err := h.getMembership(groupID, currentUser.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusForbidden, "group access denied")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed validating membership")
	}
	if actorMembership.Role != models.GroupRoleOwner && actorMembership.Role != models.GroupRoleAdmin {
		return utils.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}

	targetMembership, err := h.getMembership(groupID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "member not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading target membership")
	}
	if targetMembership.Role == models.GroupRoleOwner {
		return utils.Error(c, fiber.StatusForbidden, "cannot change owner role")
	}

	var req updateMemberRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Role != models.GroupRoleAdmin && req.Role != models.GroupRoleMember {
		return utils.Error(c, fiber.StatusBadRequest, "invalid role")
	}
	if actorMembership.Role == models.GroupRoleAdmin && req.Role != models.GroupRoleMember {
		return utils.Error(c, fiber.StatusForbidden, "admins can only set member role")
	}

	if err := h.DB.Model(&models.GroupMembership{}).Where("id = ?", targetMembership.ID).Update("role", req.Role).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating member role")
	}

	targetMembership.Role = req.Role
	return utils.Success(c, fiber.StatusOK, targetMembership)
}

func (h *GroupsHandler) getMembership(groupID, userID uuid.UUID) (*models.GroupMembership, error) {
	var membership models.GroupMembership
	err := h.DB.First(&membership, "group_id = ? AND user_id = ?", groupID, userID).Error
	if err != nil {
		return nil, err
	}
	return &membership, nil
}
