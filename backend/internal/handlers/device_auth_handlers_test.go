package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestDeviceAuthEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "device-auth-user@test.com", "password123", models.UserRoleUser)

	var deviceCode string
	var userCode string

	t.Run("POST /api/auth/device/code returns device and user codes", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/code", nil, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		deviceCode = body["device_code"].(string)
		userCode = body["user_code"].(string)
		verificationURI := body["verification_uri"].(string)
		verificationURIComplete := body["verification_uri_complete"].(string)

		if verificationURI != "http://localhost:3001/device" {
			t.Fatalf("expected verification_uri to be 'http://localhost:3001/device', got %v", verificationURI)
		}
		if !strings.HasPrefix(verificationURIComplete, "http://localhost:3001/device?code=") {
			t.Fatalf("expected verification_uri_complete to start with 'http://localhost:3001/device?code=', got %v", verificationURIComplete)
		}
	})

	t.Run("POST /api/auth/device/token authorization_pending for new code", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", deviceCode)

		resp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/token", strings.NewReader(form.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		if body["error"] != "authorization_pending" {
			t.Fatalf("expected authorization_pending error, got %v", body["error"])
		}
	})

	t.Run("POST /api/auth/device/token wrong grant_type", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("device_code", deviceCode)

		resp := performRequest(t, env.app, http.MethodPost, "/api/auth/device/token", strings.NewReader(form.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		if body["error"] != "unsupported_grant_type" {
			t.Fatalf("expected unsupported_grant_type, got %v", body["error"])
		}
	})

	t.Run("GET /api/auth/device/verify requires auth and code", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/device/verify?code="+url.QueryEscape(userCode), nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		if body["success"] != true {
			t.Fatalf("expected success=true for verify")
		}
	})

	t.Run("POST /api/auth/device/approve approves code with auth", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/device/approve", map[string]any{
			"userCode": userCode,
		}, authHeaders(token))
		assertStatus(t, resp, http.StatusOK)
	})
}
