package handlers

import (
	"net/mail"
	"strings"

	"github.com/docshare/api/internal/middleware"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/internal/services"
	"github.com/docshare/api/pkg/logger"
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB    *gorm.DB
	Audit *services.AuditService
}

func NewAuthHandler(db *gorm.DB, audit *services.AuditService) *AuthHandler {
	return &AuthHandler{DB: db, Audit: audit}
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid email")
	}
	if len(req.Password) < 8 {
		return utils.Error(c, fiber.StatusBadRequest, "password must be at least 8 characters")
	}
	if req.FirstName == "" || req.LastName == "" {
		return utils.Error(c, fiber.StatusBadRequest, "firstName and lastName are required")
	}

	var existing models.User
	if err := h.DB.First(&existing, "email = ?", req.Email).Error; err == nil {
		return utils.Error(c, fiber.StatusConflict, "email already registered")
	} else if err != gorm.ErrRecordNotFound {
		return utils.Error(c, fiber.StatusInternalServerError, "failed checking existing user")
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to hash password")
	}

	user := models.User{
		Email:        req.Email,
		PasswordHash: passwordHash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         models.UserRoleUser,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating user")
	}

	logger.Info("user_registered", map[string]interface{}{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"role":    string(user.Role),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "user.register",
		ResourceType: "user",
		ResourceID:   &user.ID,
		Details: map[string]interface{}{
			"email": user.Email,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	token, err := utils.GenerateToken(&user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating token")
	}

	return utils.Success(c, fiber.StatusCreated, fiber.Map{"token": token, "user": user})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Email == "" || req.Password == "" {
		return utils.Error(c, fiber.StatusBadRequest, "email and password are required")
	}

	var user models.User
	if err := h.DB.First(&user, "email = ?", req.Email).Error; err != nil {
		logger.Warn("login_failed_user_not_found", map[string]interface{}{
			"email": req.Email,
			"ip":    c.IP(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	if !utils.CheckPassword(req.Password, user.PasswordHash) {
		logger.Warn("login_failed_invalid_password", map[string]interface{}{
			"user_id": user.ID.String(),
			"email":   req.Email,
			"ip":      c.IP(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	logger.Info("user_login", map[string]interface{}{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"ip":      c.IP(),
	})

	hasMFA, methods := UserHasMFA(h.DB, user.ID)
	if hasMFA {
		h.Audit.LogAsync(services.AuditEntry{
			UserID:       &user.ID,
			Action:       "user.login_mfa_pending",
			ResourceType: "user",
			ResourceID:   &user.ID,
			Details: map[string]interface{}{
				"email":   user.Email,
				"methods": methods,
			},
			IPAddress: c.IP(),
			RequestID: getRequestID(c),
		})

		mfaToken, err := utils.GenerateMFAToken(user.ID, user.Email)
		if err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "failed generating MFA token")
		}
		return utils.Success(c, fiber.StatusOK, fiber.Map{
			"mfaRequired": true,
			"mfaToken":    mfaToken,
			"methods":     methods,
		})
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "user.login",
		ResourceType: "user",
		ResourceID:   &user.ID,
		Details: map[string]interface{}{
			"email": user.Email,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	token, err := utils.GenerateToken(&user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating token")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"token": token, "user": user})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	return utils.Success(c, fiber.StatusOK, user)
}

type updateMeRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	AvatarURL *string `json:"avatarURL"`
}

func (h *AuthHandler) UpdateMe(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req updateMeRequest
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

	if len(updates) == 0 {
		return utils.Error(c, fiber.StatusBadRequest, "no valid fields to update")
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", currentUser.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating user")
	}

	var updated models.User
	if err := h.DB.First(&updated, "id = ?", currentUser.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed fetching updated user")
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "user.profile_update",
		ResourceType: "user",
		ResourceID:   &currentUser.ID,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, updated)
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if len(req.NewPassword) < 8 {
		return utils.Error(c, fiber.StatusBadRequest, "newPassword must be at least 8 characters")
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", currentUser.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading user")
	}

	if !utils.CheckPassword(req.OldPassword, user.PasswordHash) {
		return utils.Error(c, fiber.StatusBadRequest, "oldPassword is incorrect")
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed hashing password")
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("password_hash", hash).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed updating password")
	}

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "user.password_change",
		ResourceType: "user",
		ResourceID:   &currentUser.ID,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "password updated"})
}
