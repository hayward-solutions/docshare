package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/utils"
	"github.com/pquerna/otp/totp"
)

func TestMFAHandler_Status_NoMFA(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "mfa-status@test.com", "password123", models.UserRoleUser)

	resp := performRequest(t, env.app, http.MethodGet, "/api/auth/mfa/status", nil, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})

	if data["mfaEnabled"].(bool) {
		t.Fatal("expected mfaEnabled to be false")
	}
	if data["totpEnabled"].(bool) {
		t.Fatal("expected totpEnabled to be false")
	}
}

func TestMFAHandler_TOTPSetup(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "totp-setup@test.com", "password123", models.UserRoleUser)

	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})

	secret := data["secret"].(string)
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}

	qrUri := data["qrUri"].(string)
	if qrUri == "" {
		t.Fatal("expected non-empty qrUri")
	}
}

func TestMFAHandler_TOTPVerifySetup(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "totp-verify@test.com", "password123", models.UserRoleUser)

	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})
	secret := data["secret"].(string)

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	resp = performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/verify-setup", map[string]interface{}{
		"code": code,
	}, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	body = decodeJSONMap(t, resp)
	data = body["data"].(map[string]interface{})

	codes := data["recoveryCodes"].([]interface{})
	if len(codes) != 10 {
		t.Fatalf("expected 10 recovery codes, got %d", len(codes))
	}

	resp = performRequest(t, env.app, http.MethodGet, "/api/auth/mfa/status", nil, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	body = decodeJSONMap(t, resp)
	data = body["data"].(map[string]interface{})

	if !data["mfaEnabled"].(bool) {
		t.Fatal("expected mfaEnabled to be true")
	}
	if !data["totpEnabled"].(bool) {
		t.Fatal("expected totpEnabled to be true")
	}
}

func TestMFAHandler_TOTPVerifySetup_InvalidCode(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "totp-invalid@test.com", "password123", models.UserRoleUser)

	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))

	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/verify-setup", map[string]interface{}{
		"code": "000000",
	}, authHeaders(token))
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestMFAHandler_LoginWithMFA(t *testing.T) {
	env := setupTestEnv(t)
	user, token := createTestUser(t, env.db, "mfa-login@test.com", "password123", models.UserRoleUser)

	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))

	var mfaCfg models.MFAConfig
	env.db.First(&mfaCfg, "user_id = ?", user.ID)

	code, _ := totp.GenerateCode(mfaCfg.TOTPSecret, time.Now())
	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/verify-setup", map[string]interface{}{
		"code": code,
	}, authHeaders(token))

	loginPayload := map[string]interface{}{
		"email":    "mfa-login@test.com",
		"password": "password123",
	}
	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", loginPayload, map[string]string{
		"Content-Type": "application/json",
	})
	assertStatus(t, resp, http.StatusOK)

	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})

	mfaRequired, ok := data["mfaRequired"].(bool)
	if !ok || !mfaRequired {
		t.Fatal("expected mfaRequired to be true")
	}

	mfaToken, ok := data["mfaToken"].(string)
	if !ok || mfaToken == "" {
		t.Fatal("expected mfaToken to be non-empty")
	}

	methods, ok := data["methods"].([]interface{})
	if !ok || len(methods) == 0 {
		t.Fatal("expected methods to be non-empty")
	}

	totpCode, _ := totp.GenerateCode(mfaCfg.TOTPSecret, time.Now())

	resp = performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/verify/totp", map[string]interface{}{
		"mfaToken": mfaToken,
		"code":     totpCode,
	}, map[string]string{"Content-Type": "application/json"})
	assertStatus(t, resp, http.StatusOK)

	body = decodeJSONMap(t, resp)
	data = body["data"].(map[string]interface{})

	if _, ok := data["token"].(string); !ok {
		t.Fatal("expected JWT token in response")
	}
	if _, ok := data["user"].(map[string]interface{}); !ok {
		t.Fatal("expected user in response")
	}
}

func TestMFAHandler_RecoveryCode(t *testing.T) {
	env := setupTestEnv(t)
	user, token := createTestUser(t, env.db, "mfa-recovery@test.com", "password123", models.UserRoleUser)

	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))

	var mfaCfg models.MFAConfig
	env.db.First(&mfaCfg, "user_id = ?", user.ID)

	code, _ := totp.GenerateCode(mfaCfg.TOTPSecret, time.Now())
	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/verify-setup", map[string]interface{}{
		"code": code,
	}, authHeaders(token))

	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})
	recoveryCodes := data["recoveryCodes"].([]interface{})
	firstCode := recoveryCodes[0].(string)

	mfaToken, _ := utils.GenerateMFAToken(user.ID, user.Email)

	resp = performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/verify/recovery", map[string]interface{}{
		"mfaToken": mfaToken,
		"code":     firstCode,
	}, map[string]string{"Content-Type": "application/json"})
	assertStatus(t, resp, http.StatusOK)

	body = decodeJSONMap(t, resp)
	data = body["data"].(map[string]interface{})

	if _, ok := data["token"].(string); !ok {
		t.Fatal("expected JWT token in response")
	}

	resp = performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/verify/recovery", map[string]interface{}{
		"mfaToken": mfaToken,
		"code":     firstCode,
	}, map[string]string{"Content-Type": "application/json"})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestMFAHandler_TOTPDisable(t *testing.T) {
	env := setupTestEnv(t)
	user, token := createTestUser(t, env.db, "totp-disable@test.com", "password123", models.UserRoleUser)

	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))

	var mfaCfg models.MFAConfig
	env.db.First(&mfaCfg, "user_id = ?", user.ID)

	code, _ := totp.GenerateCode(mfaCfg.TOTPSecret, time.Now())
	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/verify-setup", map[string]interface{}{
		"code": code,
	}, authHeaders(token))

	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/disable", map[string]interface{}{
		"password": "password123",
	}, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	resp = performRequest(t, env.app, http.MethodGet, "/api/auth/mfa/status", nil, authHeaders(token))
	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})

	if data["totpEnabled"].(bool) {
		t.Fatal("expected totpEnabled to be false after disable")
	}
}

func TestMFAHandler_RegenerateRecovery(t *testing.T) {
	env := setupTestEnv(t)
	user, token := createTestUser(t, env.db, "regen-recovery@test.com", "password123", models.UserRoleUser)

	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/setup", map[string]interface{}{}, authHeaders(token))

	var mfaCfg models.MFAConfig
	env.db.First(&mfaCfg, "user_id = ?", user.ID)

	code, _ := totp.GenerateCode(mfaCfg.TOTPSecret, time.Now())
	performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/totp/verify-setup", map[string]interface{}{
		"code": code,
	}, authHeaders(token))

	resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/mfa/recovery/regenerate", map[string]interface{}{
		"password": "password123",
	}, authHeaders(token))
	assertStatus(t, resp, http.StatusOK)

	body := decodeJSONMap(t, resp)
	data := body["data"].(map[string]interface{})

	codes := data["recoveryCodes"].([]interface{})
	if len(codes) != 10 {
		t.Fatalf("expected 10 recovery codes, got %d", len(codes))
	}
}

func init() {
	_ = json.Marshal
}
