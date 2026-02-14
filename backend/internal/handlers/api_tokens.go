package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type APITokenHandler struct {
	DB    *gorm.DB
	Audit *services.AuditService
}

func NewAPITokenHandler(db *gorm.DB, audit *services.AuditService) *APITokenHandler {
	return &APITokenHandler{DB: db, Audit: audit}
}

type createTokenRequest struct {
	Name      string  `json:"name"`
	ExpiresIn *string `json:"expiresIn"` // "30d", "90d", "365d", "never"
}

type createTokenResponse struct {
	Token    string          `json:"token"`
	APIToken models.APIToken `json:"apiToken"`
}

func (h *APITokenHandler) Create(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req createTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		return utils.Error(c, fiber.StatusBadRequest, "name is required")
	}
	if len(req.Name) > 255 {
		return utils.Error(c, fiber.StatusBadRequest, "name must be 255 characters or less")
	}

	// Limit tokens per user to prevent abuse.
	var count int64
	h.DB.Model(&models.APIToken{}).Where("user_id = ?", currentUser.ID).Count(&count)
	if count >= 25 {
		return utils.Error(c, fiber.StatusBadRequest, "maximum of 25 API tokens per user")
	}

	// Generate a cryptographically random token: dsh_<48 random hex chars>
	rawBytes := make([]byte, 24)
	if _, err := rand.Read(rawBytes); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to generate token")
	}
	rawToken := "dsh_" + hex.EncodeToString(rawBytes)
	prefix := rawToken[:8] // "dsh_xxxx" — enough to identify the token later

	// Store only the SHA-256 hash.
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn != "never" {
		var dur time.Duration
		switch *req.ExpiresIn {
		case "30d":
			dur = 30 * 24 * time.Hour
		case "90d":
			dur = 90 * 24 * time.Hour
		case "365d":
			dur = 365 * 24 * time.Hour
		default:
			return utils.Error(c, fiber.StatusBadRequest, "expiresIn must be 30d, 90d, 365d, or never")
		}
		t := time.Now().Add(dur)
		expiresAt = &t
	}

	apiToken := models.APIToken{
		UserID:    currentUser.ID,
		Name:      req.Name,
		TokenHash: tokenHash,
		Prefix:    prefix,
		ExpiresAt: expiresAt,
	}

	if err := h.DB.Create(&apiToken).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to create API token")
	}

	logger.Info("api_token_created", map[string]interface{}{
		"user_id":  currentUser.ID.String(),
		"token_id": apiToken.ID.String(),
		"name":     apiToken.Name,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "api_token.create",
		ResourceType: "api_token",
		ResourceID:   &apiToken.ID,
		Details: map[string]interface{}{
			"name":   apiToken.Name,
			"prefix": prefix,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	// Return the raw token only once — it cannot be retrieved again.
	return utils.Success(c, fiber.StatusCreated, createTokenResponse{
		Token:    rawToken,
		APIToken: apiToken,
	})
}

func (h *APITokenHandler) List(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	p := utils.ParsePagination(c)

	baseQuery := h.DB.Model(&models.APIToken{}).Where("user_id = ?", currentUser.ID)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to count API tokens")
	}

	var tokens []models.APIToken
	if err := utils.ApplyPagination(baseQuery.Order("created_at DESC"), p).Find(&tokens).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to list API tokens")
	}

	return utils.Paginated(c, tokens, p.Page, p.Limit, total)
}

func (h *APITokenHandler) Revoke(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	tokenID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid token ID")
	}

	var apiToken models.APIToken
	if err := h.DB.First(&apiToken, "id = ? AND user_id = ?", tokenID, currentUser.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusNotFound, "API token not found")
	}

	// Hard delete — revoked tokens should not be recoverable.
	if err := h.DB.Unscoped().Delete(&apiToken).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to revoke API token")
	}

	logger.Info("api_token_revoked", map[string]interface{}{
		"user_id":  currentUser.ID.String(),
		"token_id": apiToken.ID.String(),
		"name":     apiToken.Name,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "api_token.revoke",
		ResourceType: "api_token",
		ResourceID:   &apiToken.ID,
		Details: map[string]interface{}{
			"name":   apiToken.Name,
			"prefix": apiToken.Prefix,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "API token revoked"})
}
