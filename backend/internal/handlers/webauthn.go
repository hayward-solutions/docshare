package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WebAuthnHandler struct {
	DB       *gorm.DB
	WebAuthn *webauthn.WebAuthn
	Audit    *services.AuditService
}

func NewWebAuthnHandler(db *gorm.DB, wa *webauthn.WebAuthn, audit *services.AuditService) *WebAuthnHandler {
	return &WebAuthnHandler{DB: db, WebAuthn: wa, Audit: audit}
}

type webAuthnUser struct {
	user  models.User
	creds []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte {
	b, _ := u.user.ID.MarshalBinary()
	return b
}

func (u *webAuthnUser) WebAuthnName() string {
	return u.user.Email
}

func (u *webAuthnUser) WebAuthnDisplayName() string {
	return u.user.FirstName + " " + u.user.LastName
}

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.creds
}

func (h *WebAuthnHandler) loadWebAuthnUser(userID uuid.UUID) (*webAuthnUser, error) {
	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	var dbCreds []models.WebAuthnCredential
	h.DB.Where("user_id = ?", userID).Find(&dbCreds)

	creds := make([]webauthn.Credential, len(dbCreds))
	for i, dc := range dbCreds {
		var transports []protocol.AuthenticatorTransport
		if dc.Transports != "" {
			var ts []string
			json.Unmarshal([]byte(dc.Transports), &ts)
			for _, t := range ts {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}
		creds[i] = webauthn.Credential{
			ID:              dc.CredentialID,
			PublicKey:       dc.PublicKey,
			AttestationType: dc.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    dc.AAGUID,
				SignCount: dc.SignCount,
			},
			Transport: transports,
			Flags: webauthn.CredentialFlags{
				BackupEligible: dc.BackupEligible,
				BackupState:    dc.BackupState,
			},
		}
	}

	return &webAuthnUser{user: user, creds: creds}, nil
}

type registerBeginResponse struct {
	Options interface{} `json:"options"`
}

func (h *WebAuthnHandler) RegisterBegin(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	waUser, err := h.loadWebAuthnUser(user.ID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load user")
	}

	options, session, err := h.WebAuthn.BeginRegistration(waUser)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to begin registration")
	}

	sessionJSON, _ := json.Marshal(session)

	h.DB.Where("user_id = ? AND type = ?", user.ID, models.MFAChallengeRegistration).
		Delete(&models.MFAChallenge{})

	challenge := models.MFAChallenge{
		UserID:      &user.ID,
		Challenge:   []byte(session.Challenge),
		Type:        models.MFAChallengeRegistration,
		SessionData: string(sessionJSON),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	if err := h.DB.Create(&challenge).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to save challenge")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"options": options})
}

type registerFinishRequest struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

func (h *WebAuthnHandler) RegisterFinish(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req registerFinishRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		req.Name = "Passkey"
	}

	var challenge models.MFAChallenge
	if err := h.DB.Where("user_id = ? AND type = ? AND expires_at > ?",
		user.ID, models.MFAChallengeRegistration, time.Now()).
		Order("created_at DESC").First(&challenge).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "no pending registration challenge")
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(challenge.SessionData), &session); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load session")
	}

	waUser, err := h.loadWebAuthnUser(user.ID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load user")
	}

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(strings.NewReader(string(req.Response)))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid credential response")
	}

	credential, err := h.WebAuthn.CreateCredential(waUser, session, parsedResponse)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "failed to verify credential")
	}

	var transportsJSON []byte
	if len(credential.Transport) > 0 {
		ts := make([]string, len(credential.Transport))
		for i, t := range credential.Transport {
			ts[i] = string(t)
		}
		transportsJSON, _ = json.Marshal(ts)
	}

	dbCred := models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Name:            req.Name,
		Transports:      string(transportsJSON),
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}
	if err := h.DB.Create(&dbCred).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to save credential")
	}

	h.DB.Where("id = ?", challenge.ID).Delete(&models.MFAChallenge{})

	response := fiber.Map{
		"credential": dbCred,
	}

	var mfaCfg models.MFAConfig
	if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err != nil {
		mfaCfg = models.MFAConfig{UserID: user.ID}
		h.DB.Create(&mfaCfg)
	}

	if mfaCfg.RecoveryCount == 0 {
		codes, hashedCodes, err := generateRecoveryCodes(10)
		if err == nil {
			codesJSON, _ := json.Marshal(hashedCodes)
			h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
				"recovery_codes": string(codesJSON),
				"recovery_count": len(codes),
			})
			response["recoveryCodes"] = codes
		}
	}

	logger.Info("webauthn_credential_registered", map[string]interface{}{
		"user_id":       user.ID.String(),
		"credential_id": dbCred.ID.String(),
		"name":          dbCred.Name,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "mfa.passkey_registered",
		ResourceType: "webauthn_credential",
		ResourceID:   &dbCred.ID,
		Details: map[string]interface{}{
			"name": dbCred.Name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusCreated, response)
}

type verifyWebAuthnBeginRequest struct {
	MFAToken string `json:"mfaToken"`
}

func (h *WebAuthnHandler) VerifyBegin(c *fiber.Ctx) error {
	var req verifyWebAuthnBeginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	claims, err := utils.ValidateMFAToken(req.MFAToken)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "invalid or expired MFA token")
	}

	waUser, err := h.loadWebAuthnUser(claims.UserID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load user")
	}

	options, session, err := h.WebAuthn.BeginLogin(waUser)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to begin authentication")
	}

	sessionJSON, _ := json.Marshal(session)
	challenge := models.MFAChallenge{
		UserID:      &claims.UserID,
		Challenge:   []byte(session.Challenge),
		Type:        models.MFAChallengeAuthentication,
		SessionData: string(sessionJSON),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	if err := h.DB.Create(&challenge).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to save challenge")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{"options": options})
}

type verifyWebAuthnFinishRequest struct {
	MFAToken string          `json:"mfaToken"`
	Response json.RawMessage `json:"response"`
}

func (h *WebAuthnHandler) VerifyFinish(c *fiber.Ctx) error {
	var req verifyWebAuthnFinishRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	claims, err := utils.ValidateMFAToken(req.MFAToken)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "invalid or expired MFA token")
	}

	waUser, err := h.loadWebAuthnUser(claims.UserID)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load user")
	}

	var challenge models.MFAChallenge
	if err := h.DB.Where("user_id = ? AND type = ? AND expires_at > ?",
		claims.UserID, models.MFAChallengeAuthentication, time.Now()).
		Order("created_at DESC").First(&challenge).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "no pending authentication challenge")
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(challenge.SessionData), &session); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load session")
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(strings.NewReader(string(req.Response)))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid assertion response")
	}

	credential, err := h.WebAuthn.ValidateLogin(waUser, session, parsedResponse)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "passkey verification failed")
	}

	h.DB.Where("id = ?", challenge.ID).Delete(&models.MFAChallenge{})

	now := time.Now()
	h.DB.Model(&models.WebAuthnCredential{}).
		Where("user_id = ? AND credential_id = ?", claims.UserID, credential.ID).
		Updates(map[string]interface{}{
			"sign_count":   credential.Authenticator.SignCount,
			"last_used_at": now,
		})

	token, err := utils.GenerateToken(&waUser.user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating token")
	}

	logger.Info("mfa_webauthn_verified", map[string]interface{}{
		"user_id": waUser.user.ID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &waUser.user.ID,
		Action:       "user.mfa_login",
		ResourceType: "user",
		ResourceID:   &waUser.user.ID,
		Details: map[string]interface{}{
			"method": "webauthn",
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"token": token, "user": waUser.user})
}

func (h *WebAuthnHandler) LoginBegin(c *fiber.Ctx) error {
	options, session, err := h.WebAuthn.BeginDiscoverableLogin()
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to begin passkey login")
	}

	sessionJSON, _ := json.Marshal(session)
	challenge := models.MFAChallenge{
		Challenge:   []byte(session.Challenge),
		Type:        models.MFAChallengeAuthentication,
		SessionData: string(sessionJSON),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	if err := h.DB.Create(&challenge).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to save challenge")
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"options":     options,
		"challengeID": challenge.ID,
	})
}

type loginFinishRequest struct {
	ChallengeID string          `json:"challengeID"`
	Response    json.RawMessage `json:"response"`
}

func (h *WebAuthnHandler) LoginFinish(c *fiber.Ctx) error {
	var req loginFinishRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	challengeID, err := parseUUID(req.ChallengeID)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid challengeID")
	}

	var challenge models.MFAChallenge
	if err := h.DB.Where("id = ? AND type = ? AND expires_at > ?",
		challengeID, models.MFAChallengeAuthentication, time.Now()).
		First(&challenge).Error; err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "no pending login challenge")
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(challenge.SessionData), &session); err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to load session")
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(strings.NewReader(string(req.Response)))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid assertion response")
	}

	userHandle := parsedResponse.Response.UserHandle
	userID, err := uuid.FromBytes(userHandle)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid user handle")
	}

	waUser, err := h.loadWebAuthnUser(userID)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "user not found")
	}

	credential, err := h.WebAuthn.ValidateDiscoverableLogin(
		func(rawID, userHandle []byte) (webauthn.User, error) {
			return waUser, nil
		},
		session,
		parsedResponse,
	)
	if err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "passkey verification failed")
	}

	h.DB.Where("id = ?", challenge.ID).Delete(&models.MFAChallenge{})

	now := time.Now()
	h.DB.Model(&models.WebAuthnCredential{}).
		Where("user_id = ? AND credential_id = ?", userID, credential.ID).
		Updates(map[string]interface{}{
			"sign_count":   credential.Authenticator.SignCount,
			"last_used_at": now,
		})

	token, err := utils.GenerateToken(&waUser.user)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating token")
	}

	logger.Info("passkey_login", map[string]interface{}{
		"user_id": waUser.user.ID.String(),
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &waUser.user.ID,
		Action:       "user.passkey_login",
		ResourceType: "user",
		ResourceID:   &waUser.user.ID,
		IPAddress:    c.IP(),
		RequestID:    getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"token": token, "user": waUser.user})
}

func (h *WebAuthnHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var creds []models.WebAuthnCredential
	h.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Find(&creds)

	return utils.Success(c, fiber.StatusOK, creds)
}

type renamePasskeyRequest struct {
	Name string `json:"name"`
}

func (h *WebAuthnHandler) Rename(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	credID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid credential ID")
	}

	var req renamePasskeyRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		return utils.Error(c, fiber.StatusBadRequest, "name is required")
	}

	result := h.DB.Model(&models.WebAuthnCredential{}).
		Where("id = ? AND user_id = ?", credID, user.ID).
		Update("name", req.Name)
	if result.RowsAffected == 0 {
		return utils.Error(c, fiber.StatusNotFound, "passkey not found")
	}

	var cred models.WebAuthnCredential
	h.DB.First(&cred, "id = ?", credID)

	return utils.Success(c, fiber.StatusOK, cred)
}

func (h *WebAuthnHandler) Delete(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	credID, err := parseUUID(c.Params("id"))
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid credential ID")
	}

	var cred models.WebAuthnCredential
	if err := h.DB.First(&cred, "id = ? AND user_id = ?", credID, user.ID).Error; err != nil {
		return utils.Error(c, fiber.StatusNotFound, "passkey not found")
	}

	if err := h.DB.Unscoped().Delete(&cred).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to delete passkey")
	}

	var remainingCreds int64
	h.DB.Model(&models.WebAuthnCredential{}).Where("user_id = ?", user.ID).Count(&remainingCreds)

	if remainingCreds == 0 {
		var mfaCfg models.MFAConfig
		if err := h.DB.First(&mfaCfg, "user_id = ?", user.ID).Error; err == nil && !mfaCfg.TOTPEnabled {
			h.DB.Model(&mfaCfg).Updates(map[string]interface{}{
				"recovery_codes": "",
				"recovery_count": 0,
			})
		}
	}

	logger.Info("webauthn_credential_deleted", map[string]interface{}{
		"user_id":       user.ID.String(),
		"credential_id": credID.String(),
		"name":          cred.Name,
	})

	h.Audit.LogAsync(services.AuditEntry{
		UserID:       &user.ID,
		Action:       "mfa.passkey_removed",
		ResourceType: "webauthn_credential",
		ResourceID:   &cred.ID,
		Details: map[string]interface{}{
			"name": cred.Name,
		},
		IPAddress: c.IP(),
		RequestID: getRequestID(c),
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"message": "passkey removed"})
}
