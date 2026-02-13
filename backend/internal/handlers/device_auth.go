package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	deviceCodeLifetime = 15 * time.Minute
	devicePollInterval = 5
	userCodeAlphabet   = "BCDFGHJKLMNPQRSTVWXZ"
	userCodeLength     = 8
)

type DeviceAuthHandler struct {
	DB    *gorm.DB
	Audit *services.AuditService
}

func NewDeviceAuthHandler(db *gorm.DB, audit *services.AuditService) *DeviceAuthHandler {
	return &DeviceAuthHandler{DB: db, Audit: audit}
}

// RequestCode implements the Device Authorization Endpoint (RFC 8628 Section 3.1-3.2).
// The golang.org/x/oauth2 client POSTs form-encoded data and expects standard OAuth2 JSON.
func (h *DeviceAuthHandler) RequestCode(c *fiber.Ctx) error {
	rawDeviceCode, err := generateRandomHex(32)
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "failed to generate device code")
	}

	userCode, err := generateUserCode()
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "failed to generate user code")
	}

	hash := sha256.Sum256([]byte(rawDeviceCode))
	deviceCodeHash := hex.EncodeToString(hash[:])

	expiresAt := time.Now().Add(deviceCodeLifetime)

	dc := models.DeviceCode{
		DeviceCodeHash: deviceCodeHash,
		UserCode:       userCode,
		ExpiresAt:      expiresAt,
		Interval:       devicePollInterval,
		Status:         models.DeviceCodePending,
	}

	if err := h.DB.Create(&dc).Error; err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "failed to create device code")
	}

	verificationURI := frontendURL() + "/device"
	verificationURIComplete := verificationURI + "?code=" + userCode

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"device_code":               rawDeviceCode,
		"user_code":                 formatUserCode(userCode),
		"verification_uri":          verificationURI,
		"verification_uri_complete": verificationURIComplete,
		"expires_in":                int(time.Until(expiresAt).Seconds()),
		"interval":                  devicePollInterval,
	})
}

// PollToken implements the Device Access Token Endpoint (RFC 8628 Section 3.4-3.5).
// The golang.org/x/oauth2 client POSTs form-encoded data and expects standard OAuth2 token JSON.
func (h *DeviceAuthHandler) PollToken(c *fiber.Ctx) error {
	grantType := c.FormValue("grant_type")
	if grantType != "urn:ietf:params:oauth:grant-type:device_code" {
		return oauthError(c, fiber.StatusBadRequest, "unsupported_grant_type", "expected device_code grant type")
	}

	rawDeviceCode := c.FormValue("device_code")
	if rawDeviceCode == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "device_code is required")
	}

	hash := sha256.Sum256([]byte(rawDeviceCode))
	deviceCodeHash := hex.EncodeToString(hash[:])

	var dc models.DeviceCode
	if err := h.DB.First(&dc, "device_code_hash = ?", deviceCodeHash).Error; err != nil {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "unknown device code")
	}

	if time.Now().After(dc.ExpiresAt) {
		h.DB.Model(&dc).Update("status", models.DeviceCodeExpired)
		return oauthError(c, fiber.StatusBadRequest, "expired_token", "the device code has expired")
	}

	switch dc.Status {
	case models.DeviceCodePending:
		return oauthError(c, fiber.StatusBadRequest, "authorization_pending", "the user has not yet approved")

	case models.DeviceCodeDenied:
		return oauthError(c, fiber.StatusBadRequest, "access_denied", "the user denied the request")

	case models.DeviceCodeApproved:
		if dc.UserID == nil {
			return oauthError(c, fiber.StatusInternalServerError, "server_error", "approved but no user attached")
		}

		var user models.User
		if err := h.DB.First(&user, "id = ?", *dc.UserID).Error; err != nil {
			return oauthError(c, fiber.StatusInternalServerError, "server_error", "user not found")
		}

		token, err := utils.GenerateToken(&user)
		if err != nil {
			return oauthError(c, fiber.StatusInternalServerError, "server_error", "failed to generate token")
		}

		h.DB.Unscoped().Delete(&dc)

		logger.Info("device_flow_token_issued", map[string]interface{}{
			"user_id": user.ID.String(),
		})

		h.Audit.LogAsync(services.AuditEntry{
			UserID:       dc.UserID,
			Action:       "auth.device_flow_login",
			ResourceType: "user",
			ResourceID:   dc.UserID,
			IPAddress:    c.IP(),
			RequestID:    getRequestID(c),
		})

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   86400,
		})

	default:
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "invalid device code status")
	}
}

// Approve is called by the browser-authenticated user to approve a device code.
// This uses DocShare's standard response format since the frontend calls it.
func (h *DeviceAuthHandler) Approve(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req struct {
		UserCode string `json:"userCode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	code := normalizeUserCode(req.UserCode)
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "userCode is required")
	}

	var dc models.DeviceCode
	if err := h.DB.First(&dc, "user_code = ? AND status = ?", code, models.DeviceCodePending).Error; err != nil {
		return utils.Error(c, fiber.StatusNotFound, "no pending device code found for this code")
	}

	if time.Now().After(dc.ExpiresAt) {
		h.DB.Model(&dc).Update("status", models.DeviceCodeExpired)
		return utils.Error(c, fiber.StatusGone, "this device code has expired")
	}

	if err := h.DB.Model(&dc).Updates(map[string]interface{}{
		"status":  models.DeviceCodeApproved,
		"user_id": currentUser.ID,
	}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to approve device code")
	}

	logger.Info("device_flow_approved", map[string]interface{}{
		"user_id":   currentUser.ID.String(),
		"user_code": code,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &currentUser.ID,
		Action:       "auth.device_flow_approve",
		ResourceType: "device_code",
		ResourceID:   &dc.ID,
		Details: map[string]interface{}{
			"user_code": code,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "device authorized"})
}

// Verify looks up a user_code and returns its status. Called by the frontend to pre-fill the code.
func (h *DeviceAuthHandler) Verify(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := normalizeUserCode(c.Query("code"))
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code query parameter is required")
	}

	var dc models.DeviceCode
	if err := h.DB.First(&dc, "user_code = ?", code).Error; err != nil {
		return utils.Error(c, fiber.StatusNotFound, "device code not found")
	}

	expired := time.Now().After(dc.ExpiresAt)

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"userCode":  formatUserCode(dc.UserCode),
		"status":    dc.Status,
		"expired":   expired,
		"expiresAt": dc.ExpiresAt,
	})
}

// CleanupExpiredDeviceCodes removes device codes that have been expired for over an hour.
func CleanupExpiredDeviceCodes(db *gorm.DB) {
	cutoff := time.Now().Add(-1 * time.Hour)
	db.Unscoped().Where("expires_at < ?", cutoff).Delete(&models.DeviceCode{})
}

func oauthError(c *fiber.Ctx, status int, errorCode string, description string) error {
	return c.Status(status).JSON(fiber.Map{
		"error":             errorCode,
		"error_description": description,
	})
}

func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateUserCode() (string, error) {
	code := make([]byte, userCodeLength)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(userCodeAlphabet))))
		if err != nil {
			return "", err
		}
		code[i] = userCodeAlphabet[n.Int64()]
	}
	return string(code), nil
}

func formatUserCode(code string) string {
	if len(code) == userCodeLength {
		return code[:4] + "-" + code[4:]
	}
	return code
}

func normalizeUserCode(input string) string {
	s := strings.ToUpper(strings.TrimSpace(input))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func frontendURL() string {
	return "http://localhost:3001"
}
