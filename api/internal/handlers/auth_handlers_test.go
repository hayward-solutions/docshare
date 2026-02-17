package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestAuthEndpoints(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("POST /api/auth/register", func(t *testing.T) {
		t.Run("success creates user and token", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
				"email":     "auth-register-success@test.com",
				"password":  "password123",
				"firstName": "Auth",
				"lastName":  "Success",
			}, nil)
			body := decodeJSONMap(t, resp)

			assertStatus(t, resp, http.StatusCreated)
			if success, _ := body["success"].(bool); !success {
				t.Fatalf("expected success response, got %+v", body)
			}
			data := body["data"].(map[string]any)
			if _, ok := data["token"].(string); !ok {
				t.Fatalf("expected token string in response")
			}
		})

		t.Run("duplicate email returns conflict", func(t *testing.T) {
			_, _ = createTestUser(t, env.db, "auth-register-duplicate@test.com", "password123", models.UserRoleUser)
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
				"email":     "auth-register-duplicate@test.com",
				"password":  "password123",
				"firstName": "Dup",
				"lastName":  "User",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusConflict)
			assertEnvelopeError(t, body, "email already registered")
		})

		t.Run("short password returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
				"email":     "auth-register-short@test.com",
				"password":  "short",
				"firstName": "Short",
				"lastName":  "Password",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "password must be at least 8 characters")
		})

		t.Run("missing fields returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "invalid email")
		})
	})

	t.Run("POST /api/auth/login", func(t *testing.T) {
		_, _ = createTestUser(t, env.db, "auth-login@test.com", "password123", models.UserRoleUser)

		t.Run("success returns token", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", map[string]any{
				"email":    "auth-login@test.com",
				"password": "password123",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)
			data := body["data"].(map[string]any)
			if _, ok := data["token"].(string); !ok {
				t.Fatalf("expected token in login response")
			}
		})

		t.Run("wrong password returns unauthorized", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", map[string]any{
				"email":    "auth-login@test.com",
				"password": "wrongpassword",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
			assertEnvelopeError(t, body, "invalid credentials")
		})

		t.Run("non-existent email returns unauthorized", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", map[string]any{
				"email":    "missing-user@test.com",
				"password": "password123",
			}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
			assertEnvelopeError(t, body, "invalid credentials")
		})

		t.Run("empty body returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", map[string]any{}, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "email and password are required")
		})
	})

	t.Run("GET /api/auth/me", func(t *testing.T) {
		_, token := createTestUser(t, env.db, "auth-me@test.com", "password123", models.UserRoleUser)

		t.Run("success with valid token", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/auth/me", nil, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)
			if success, _ := body["success"].(bool); !success {
				t.Fatalf("expected success=true")
			}
		})

		t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/auth/me", nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
			assertEnvelopeError(t, body, "missing authorization header")
		})
	})

	t.Run("PUT /api/auth/me", func(t *testing.T) {
		_, token := createTestUser(t, env.db, "auth-update-me@test.com", "password123", models.UserRoleUser)

		t.Run("success updates names", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
				"firstName": "Updated",
				"lastName":  "Person",
			}, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)
			data := body["data"].(map[string]any)
			if data["firstName"] != "Updated" {
				t.Fatalf("expected updated firstName")
			}
		})

		t.Run("empty firstName returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
				"firstName": "   ",
			}, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "firstName cannot be empty")
		})

		t.Run("no fields returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{}, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "no valid fields to update")
		})
	})

	t.Run("PUT /api/auth/password", func(t *testing.T) {
		_, token := createTestUser(t, env.db, "auth-password@test.com", "password123", models.UserRoleUser)

		t.Run("success changes password", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/password", map[string]any{
				"oldPassword": "password123",
				"newPassword": "newpassword123",
			}, authHeaders(token))
			assertStatus(t, resp, http.StatusOK)
		})

		t.Run("wrong old password returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/password", map[string]any{
				"oldPassword": "wrong-old",
				"newPassword": "newpassword123",
			}, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "oldPassword is incorrect")
		})

		t.Run("short new password returns bad request", func(t *testing.T) {
			resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/password", map[string]any{
				"oldPassword": "password123",
				"newPassword": "short",
			}, authHeaders(token))
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "newPassword must be at least 8 characters")
		})
	})
}
