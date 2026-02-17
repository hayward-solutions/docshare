package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

type MFAHandler struct {
	DB    *gorm.DB
	Audit *services.AuditService
}

func NewMFAHandler(db *gorm.DB, audit *services.AuditService) *MFAHandler {
	return &MFAHandler{DB: db, Audit: audit}
}

func (h *MFAHandler) Status(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var mfaCfg models.MFAConfig
	hasMFA := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error == nil

	var credCount int64
	h.DB.Model(&models.WebAuthnCredential{}).Where("user_id = ?", user.ID).Count(&credCount)

	totpEnabled := hasMFA && mfaCfg.TOTPEnabled
	webauthnEnabled := credCount > 0
	mfaEnabled := totpEnabled || webauthnEnabled

	var totpVerifiedAt *time.Time
	if hasMFA {
		totpVerifiedAt = mfaCfg.TOTPVerifiedAt
	}

	recoveryCount := 0
	if hasMFA {
		recoveryCount = mfaCfg.RecoveryCount
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"mfaEnabled":               mfaEnabled,
		"totpEnabled":              totpEnabled,
		"totpVerifiedAt":           totpVerifiedAt,
		"webauthnEnabled":          webauthnEnabled,
		"webauthnCredentialsCount": credCount,
		"recoveryCodesRemaining":   recoveryCount,
	})
}

func (h *MFAHandler) TOTPSetup(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var existing models.MFAConfig
	if err := h.DB.First(&existing, "user_id = ?", user.ID).Error; err == nil && existing.TOTPEnabled {
		return utils.Error(c, fiber.StatusConflict, "TOTP is already enabled")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "DocShare",
		AccountName: user.Email,
	})
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to generate TOTP secret")
	}

	encryptedSecret, err := utils.EncryptAESGCM(key.Secret())
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to encrypt TOTP secret")
	}

	if existing.ID != [16]byte{} {
		if err := h.DB.Model(&existing).Updates(map[string]interface{}{
			"totp_secret":      encryptedSecret,
			"totp_enabled":     false,
			"totp_verified_at": nil,
		}).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "failed to update TOTP config")
		}
	} else {
		mfaCfg := models.MFAConfig{
			UserID:     user.ID,
			TOTPSecret: encryptedSecret,
		}
		if err := h.DB.Create(&mfaCfg).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "failed to save TOTP config")
		}
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"secret": key.Secret(),
		"qrUri":  key.URL(),
	})
}

type verifyTOTPSetupRequest struct {
	Code string `json:"code"`
}

func (h *MFAHandler) TOTPVerifySetup(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req verifyTOTPSetupRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var mfaCfg models.MFAConfig
	if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "TOTP setup not started")
	}

	if mfaCfg.TOTPEnabled {
		return utils.Error(c, fiber.StatusConflict, "TOTP is already enabled")
	}

	totpSecret := utils.DecryptOrPlaintext(mfaCfg.TOTPSecret)
	if !totp.Validate(req.Code, totpSecret) {
		return utils.Error(c, fiber.StatusBadRequest, "invalid TOTP code")
	}

	codes, hashedCodes, err := generateRecoveryCodes(10)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to generate recovery codes")
	}

	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to serialize recovery codes")
	}
	now := time.Now()
	if err := h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
		"totp_enabled":     true,
		"totp_verified_at": now,
		"recovery_codes":   string(codesJSON),
		"recovery_count":   len(codes),
	}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to enable TOTP")
	}

	logger.Info("mfa_totp_enabled", map[string]interface{}{
		"user_id": user.ID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "mfa.totp_enabled",
		ResourceType: "user",
		ResourceID:   &user.ID,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"recoveryCodes": codes,
	})
}

type disableTOTPRequest struct {
	Password string `json:"password"`
	TOTPCode string `json:"totpCode"`
}

func (h *MFAHandler) TOTPDisable(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req disableTOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	var dbUser models.User
	if err := h.DB.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load user")
	}

	var mfaCfg models.MFAConfig
	if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "MFA is not configured")
	}

	isSSOUser := dbUser.AuthProvider != nil && *dbUser.AuthProvider != ""
	hasTOTP := mfaCfg.TOTPEnabled

	if isSSOUser {
		if !hasTOTP || req.TOTPCode == "" {
			return utils.Error(c, fiber.StatusBadRequest, "TOTP code required for SSO users")
		}
		totpSecret := utils.DecryptOrPlaintext(mfaCfg.TOTPSecret)
		if !totp.Validate(req.TOTPCode, totpSecret) {
			return utils.Error(c, fiber.StatusBadRequest, "invalid TOTP code")
		}
	} else {
		if req.Password == "" {
			return utils.Error(c, fiber.StatusBadRequest, "password is required")
		}
		if !utils.CheckPassword(req.Password, dbUser.PasswordHash) {
			return utils.Error(c, fiber.StatusBadRequest, "invalid password")
		}
	}

	if err := h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
		"totp_enabled":     false,
		"totp_secret":      "",
		"totp_verified_at": nil,
	}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to disable TOTP")
	}

	var credCount int64
	h.DB.Model(&models.WebAuthnCredential{}).Where("user_id = ?", user.ID).Count(&credCount)
	if credCount == 0 {
		if err := h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
			"recovery_codes": "",
			"recovery_count": 0,
		}).Error; err != nil {
			return utils.Error(c, fiber.StatusInternalServerError, "failed to clear recovery codes")
		}
	}

	logger.Info("mfa_totp_disabled", map[string]interface{}{
		"user_id": user.ID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "mfa.totp_disabled",
		ResourceType: "user",
		ResourceID:   &user.ID,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "TOTP disabled"})
}

type verifyMFATOTPRequest struct {
	MFAToken string `json:"mfaToken"`
	Code     string `json:"code"`
}

func (h *MFAHandler) VerifyTOTP(c *fiber.Ctx) error {
	var req verifyMFATOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.MFAToken == "" || req.Code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "mfaToken and code are required")
	}

	claims, err := utils.ValidateMFAToken(req.MFAToken)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "invalid or expired MFA token")
	}

	if !utils.IsJTIValid(claims.JTI) {
		return utils.Error(c, fiber.StatusUnauthorized, "MFA token already used")
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "user not found")
	}

	var mfaCfg models.MFAConfig
	if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err != nil || !mfaCfg.TOTPEnabled {
		return utils.Error(c, fiber.StatusBadRequest, "TOTP is not enabled")
	}

	totpSecret := utils.DecryptOrPlaintext(mfaCfg.TOTPSecret)
	if !totp.Validate(req.Code, totpSecret) {
		return utils.Error(c, fiber.StatusUnauthorized, "invalid TOTP code")
	}

	utils.ConsumeJTI(claims.JTI)

	token, err := utils.GenerateToken(&user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating token")
	}

	logger.Info("mfa_totp_verified", map[string]interface{}{
		"user_id": user.ID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "user.mfa_login",
		ResourceType: "user",
		ResourceID:   &user.ID,
		Details: map[string]interface{}{
			"method": "totp",
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"token": token, "user": user})
}

type verifyRecoveryRequest struct {
	MFAToken string `json:"mfaToken"`
	Code     string `json:"code"`
}

func (h *MFAHandler) VerifyRecovery(c *fiber.Ctx) error {
	var req verifyRecoveryRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.MFAToken == "" || req.Code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "mfaToken and code are required")
	}

	claims, err := utils.ValidateMFAToken(req.MFAToken)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "invalid or expired MFA token")
	}

	if !utils.IsJTIValid(claims.JTI) {
		return utils.Error(c, fiber.StatusUnauthorized, "MFA token already used")
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "user not found")
	}

	var mfaCfg models.MFAConfig
	if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "MFA is not configured")
	}

	var storedCodes []string
	if err := json.Unmarshal([]byte(mfaCfg.RecoveryCodes), &storedCodes); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load recovery codes")
	}

	matchIndex := -1
	for i, hashed := range storedCodes {
		if utils.CheckPassword(req.Code, hashed) {
			matchIndex = i
			break
		}
	}

	if matchIndex == -1 {
		return utils.Error(c, fiber.StatusUnauthorized, "invalid recovery code")
	}

	storedCodes = append(storedCodes[:matchIndex], storedCodes[matchIndex+1:]...)
	updatedJSON, err := json.Marshal(storedCodes)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to serialize recovery codes")
	}
	if err := h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
		"recovery_codes": string(updatedJSON),
		"recovery_count": len(storedCodes),
	}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to update recovery codes")
	}

	utils.ConsumeJTI(claims.JTI)

	token, err := utils.GenerateToken(&user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating token")
	}

	logger.Info("mfa_recovery_used", map[string]interface{}{
		"user_id":         user.ID.String(),
		"remaining_codes": len(storedCodes),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "user.mfa_recovery",
		ResourceType: "user",
		ResourceID:   &user.ID,
		Details: map[string]interface{}{
			"remaining_codes": len(storedCodes),
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"token": token, "user": user})
}

type regenerateRecoveryRequest struct {
	Password string `json:"password"`
	TOTPCode string `json:"totpCode"`
}

func (h *MFAHandler) RegenerateRecovery(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req regenerateRecoveryRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	var dbUser models.User
	if err := h.DB.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load user")
	}

	var mfaCfg models.MFAConfig
	if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "MFA is not configured")
	}

	isSSOUser := dbUser.AuthProvider != nil && *dbUser.AuthProvider != ""
	hasTOTP := mfaCfg.TOTPEnabled

	if isSSOUser {
		if !hasTOTP || req.TOTPCode == "" {
			return utils.Error(c, fiber.StatusBadRequest, "TOTP code required for SSO users")
		}
		totpSecret := utils.DecryptOrPlaintext(mfaCfg.TOTPSecret)
		if !totp.Validate(req.TOTPCode, totpSecret) {
			return utils.Error(c, fiber.StatusBadRequest, "invalid TOTP code")
		}
	} else {
		if req.Password == "" {
			return utils.Error(c, fiber.StatusBadRequest, "password is required")
		}
		if !utils.CheckPassword(req.Password, dbUser.PasswordHash) {
			return utils.Error(c, fiber.StatusBadRequest, "invalid password")
		}
	}

	codes, hashedCodes, err := generateRecoveryCodes(10)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to generate recovery codes")
	}

	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to serialize recovery codes")
	}
	if err := h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
		"recovery_codes": string(codesJSON),
		"recovery_count": len(codes),
	}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to update recovery codes")
	}

	logger.Info("mfa_recovery_regenerated", map[string]interface{}{
		"user_id": user.ID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "mfa.recovery_regenerated",
		ResourceType: "user",
		ResourceID:   &user.ID,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"recoveryCodes": codes,
	})
}

func generateRecoveryCodes(count int) (plaintextCodes []string, hashedCodes []string, err error) {
	for i := 0; i < count; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		code := hex.EncodeToString(b)
		plaintextCodes = append(plaintextCodes, code)

		hashed, err := utils.HashPassword(code)
		if err != nil {
			return nil, nil, err
		}
		hashedCodes = append(hashedCodes, hashed)
	}
	return plaintextCodes, hashedCodes, nil
}

func UserHasMFA(db *gorm.DB, userID interface{}) (bool, []string) {
	var mfaCfg models.MFAConfig
	hasTOTP := false
	if err := db.First(&mfaCfg, "user_id = ?", userID).Error; err == nil {
		hasTOTP = mfaCfg.TOTPEnabled
	}

	var credCount int64
	db.Model(&models.WebAuthnCredential{}).Where("user_id = ?", userID).Count(&credCount)
	hasWebAuthn := credCount > 0

	if !hasTOTP && !hasWebAuthn {
		return false, nil
	}

	var methods []string
	if hasTOTP {
		methods = append(methods, "totp")
	}
	if hasWebAuthn {
		methods = append(methods, "webauthn")
	}
	return true, methods
}

func CleanupExpiredMFAChallenges(db *gorm.DB) {
	db.Where("expires_at < ?", time.Now()).Delete(&models.MFAChallenge{})
}
