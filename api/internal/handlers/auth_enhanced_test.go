package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestAuthEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("POST /api/auth/register normalizes email case", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":     "UPPER@TEST.COM",
			"password":  "password123",
			"firstName": "Upper",
			"lastName":  "Case",
		}, nil)
		assertStatus(t, resp, http.StatusCreated)

		resp2 := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", map[string]any{
			"email":    "upper@test.com",
			"password": "password123",
		}, nil)
		assertStatus(t, resp2, http.StatusOK)
	})

	t.Run("POST /api/auth/register invalid email format", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":     "not-an-email",
			"password":  "password123",
			"firstName": "Bad",
			"lastName":  "Email",
		}, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid email")
	})

	t.Run("POST /api/auth/register missing firstName", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":    "no-name@test.com",
			"password": "password123",
			"lastName": "User",
		}, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "firstName and lastName are required")
	})

	t.Run("POST /api/auth/register missing lastName", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":     "no-lastname@test.com",
			"password":  "password123",
			"firstName": "User",
		}, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "firstName and lastName are required")
	})

	t.Run("POST /api/auth/register trims whitespace", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":     "  trimmed@test.com  ",
			"password":  "password123",
			"firstName": "  Trimmed  ",
			"lastName":  "  User  ",
		}, nil)
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/auth/login with email whitespace and case", func(t *testing.T) {
		_, _ = createTestUser(t, env.db, "logincase@test.com", "password123", models.UserRoleUser)

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/login", map[string]any{
			"email":    "  LOGINCASE@TEST.COM  ",
			"password": "password123",
		}, nil)
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("GET /api/auth/me with invalid token", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/me", nil, map[string]string{
			"Authorization": "Bearer invalid-token-here",
		})
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("PUT /api/auth/me update avatarURL", func(t *testing.T) {
		_, token := createTestUser(t, env.db, "avatar-update@test.com", "password123", models.UserRoleUser)

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
			"avatarURL": "https://example.com/avatar.png",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].(map[string]any)
		if data["avatarURL"] != "https://example.com/avatar.png" {
			t.Fatalf("expected updated avatarURL")
		}
	})

	t.Run("PUT /api/auth/me clear avatarURL with empty string", func(t *testing.T) {
		_, token := createTestUser(t, env.db, "avatar-clear@test.com", "password123", models.UserRoleUser)

		performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
			"avatarURL": "https://example.com/avatar.png",
		}, authHeaders(token))

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
			"avatarURL": "",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].(map[string]any)
		if data["avatarURL"] != nil {
			t.Fatalf("expected null avatarURL, got %v", data["avatarURL"])
		}
	})

	t.Run("PUT /api/auth/me empty lastName returns bad request", func(t *testing.T) {
		_, token := createTestUser(t, env.db, "empty-lastname@test.com", "password123", models.UserRoleUser)

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
			"lastName": "   ",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "lastName cannot be empty")
	})

	t.Run("PUT /api/auth/me requires authentication", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/me", map[string]any{
			"firstName": "Test",
		}, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("PUT /api/auth/password requires authentication", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/auth/password", map[string]any{
			"oldPassword": "password123",
			"newPassword": "newpassword123",
		}, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("POST /api/auth/register password exactly 8 chars succeeds", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":     "exact8@test.com",
			"password":  "12345678",
			"firstName": "Min",
			"lastName":  "Pass",
		}, nil)
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/auth/register password 7 chars fails", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/register", map[string]any{
			"email":     "short7@test.com",
			"password":  "1234567",
			"firstName": "Short",
			"lastName":  "Pass",
		}, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "password must be at least 8 characters")
	})
}
