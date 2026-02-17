package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestSSOEndpoints(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("GET /api/auth/sso/providers", func(t *testing.T) {
		t.Run("returns empty when no providers configured", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/auth/sso/providers", nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)
			if success, _ := body["success"].(bool); !success {
				t.Fatalf("expected success=true, got %+v", body)
			}
			data := body["data"].([]any)
			if len(data) != 0 {
				t.Fatalf("expected empty providers list, got %d items", len(data))
			}
		})
	})

	t.Run("POST /api/auth/sso/ldap/login", func(t *testing.T) {
		t.Run("returns unauthorized when LDAP not configured", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/sso/ldap/login", map[string]any{
				"username": "testuser",
				"password": "testpass",
			}, nil)
			decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
		})

		t.Run("returns bad request for empty credentials", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/sso/ldap/login", map[string]any{
				"username": "",
				"password": "",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "username and password are required")
		})

		t.Run("returns bad request for missing password", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/sso/ldap/login", map[string]any{
				"username": "testuser",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "username and password are required")
		})
	})

	t.Run("GET /api/auth/sso/linked-accounts", func(t *testing.T) {
		t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/auth/sso/linked-accounts", nil, nil)
			decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
		})

		t.Run("success returns empty list for new user", func(t *testing.T) {
			_, token := createTestUser(t, env.db, "sso-link-account@test.com", "password123", models.UserRoleUser)
			resp := performRequest(t, env.app, http.MethodGet, "/api/auth/sso/linked-accounts", nil, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)
			if success, _ := body["success"].(bool); !success {
				t.Fatalf("expected success=true")
			}
		})
	})

	t.Run("DELETE /api/auth/sso/linked-accounts/:id", func(t *testing.T) {
		t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodDelete, "/api/auth/sso/linked-accounts/some-id", nil, nil)
			decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
		})

		t.Run("success even for invalid UUID (handler handles gracefully)", func(t *testing.T) {
			_, token := createTestUser(t, env.db, "sso-unlink@test.com", "password123", models.UserRoleUser)
			resp := performRequest(t, env.app, http.MethodDelete, "/api/auth/sso/linked-accounts/invalid-uuid", nil, authHeaders(token))
			assertStatus(t, resp, http.StatusOK)
		})

		t.Run("success for non-existent account (idempotent)", func(t *testing.T) {
			_, token := createTestUser(t, env.db, "sso-unlink2@test.com", "password123", models.UserRoleUser)
			resp := performRequest(t, env.app, http.MethodDelete, "/api/auth/sso/linked-accounts/00000000-0000-0000-0000-000000000000", nil, authHeaders(token))
			assertStatus(t, resp, http.StatusOK)
		})
	})
}
