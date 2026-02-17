package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestDeviceAuthEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "device-enh-user@test.com", "password123", models.UserRoleUser)

	t.Run("full flow: request code, approve, then poll for token", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/code", nil, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		deviceCode := body["device_code"].(string)
		userCode := body["user_code"].(string)

		approveResp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/device/approve", map[string]any{
			"userCode": userCode,
		}, authHeaders(token))
		assertStatus(t, approveResp, http.StatusOK)

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", deviceCode)

		tokenResp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/token", strings.NewReader(form.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		tokenBody := decodeJSONMap(t, tokenResp)
		assertStatus(t, tokenResp, http.StatusOK)

		if tokenBody["access_token"] == nil || tokenBody["access_token"] == "" {
			t.Fatal("expected access_token in response")
		}
		if tokenBody["token_type"] != "Bearer" {
			t.Fatalf("expected token_type Bearer, got %v", tokenBody["token_type"])
		}
	})

	t.Run("POST /api/auth/device/token missing device_code", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		resp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/token", strings.NewReader(form.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		if body["error"] != "invalid_request" {
			t.Fatalf("expected invalid_request error, got %v", body["error"])
		}
	})

	t.Run("POST /api/auth/device/token unknown device code", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", "nonexistent-device-code-12345")

		resp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/token", strings.NewReader(form.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		if body["error"] != "invalid_grant" {
			t.Fatalf("expected invalid_grant error, got %v", body["error"])
		}
	})

	t.Run("POST /api/auth/device/approve empty userCode", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/device/approve", map[string]any{
			"userCode": "",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "userCode is required")
	})

	t.Run("POST /api/auth/device/approve non-existent code", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/device/approve", map[string]any{
			"userCode": "ZZZZZZZZ",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "no pending device code found for this code")
	})

	t.Run("POST /api/auth/device/approve requires authentication", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/device/approve", map[string]any{
			"userCode": "TESTCODE",
		}, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/auth/device/verify missing code", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/device/verify", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "code query parameter is required")
	})

	t.Run("GET /api/auth/device/verify non-existent code", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/device/verify?code=NOTEXIST", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "device code not found")
	})

	t.Run("GET /api/auth/device/verify requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/device/verify?code=TEST", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestDeviceAuthHelpers(t *testing.T) {
	t.Run("formatUserCode formats 8 char code", func(t *testing.T) {
		result := formatUserCode("ABCDEFGH")
		if result != "ABCD-EFGH" {
			t.Errorf("expected ABCD-EFGH, got %s", result)
		}
	})

	t.Run("formatUserCode returns short codes unchanged", func(t *testing.T) {
		result := formatUserCode("ABC")
		if result != "ABC" {
			t.Errorf("expected ABC, got %s", result)
		}
	})

	t.Run("normalizeUserCode strips dashes and spaces", func(t *testing.T) {
		result := normalizeUserCode("abcd-efgh")
		if result != "ABCDEFGH" {
			t.Errorf("expected ABCDEFGH, got %s", result)
		}

		result = normalizeUserCode("  ABCD EFGH  ")
		if result != "ABCDEFGH" {
			t.Errorf("expected ABCDEFGH, got %s", result)
		}
	})

	t.Run("generateUserCode produces correct length", func(t *testing.T) {
		code, err := generateUserCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(code) != userCodeLength {
			t.Errorf("expected code length %d, got %d", userCodeLength, len(code))
		}
	})

	t.Run("generateRandomHex produces correct length", func(t *testing.T) {
		hex, err := generateRandomHex(16)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hex) != 32 {
			t.Errorf("expected hex length 32, got %d", len(hex))
		}
	})
}
