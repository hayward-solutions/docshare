package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestAPITokenEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "api-token-user@test.com", "password123", models.UserRoleUser)

	var tokenID string

	t.Run("POST /api/auth/tokens/ creates token", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name":      "CLI Token",
			"expiresIn": "30d",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)

		data := body["data"].(map[string]any)
		apiToken := data["apiToken"].(map[string]any)
		tokenID = apiToken["id"].(string)
		if _, ok := data["token"].(string); !ok {
			t.Fatalf("expected raw token in create response")
		}
	})

	t.Run("POST /api/auth/tokens/ missing name", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "name is required")
	})

	t.Run("POST /api/auth/tokens/ invalid expiresIn", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name":      "Bad Expiry",
			"expiresIn": "10d",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "expiresIn must be 30d, 90d, 365d, or never")
	})

	t.Run("GET /api/auth/tokens/ lists tokens", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/tokens/", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected at least one API token")
		}
	})

	t.Run("DELETE /api/auth/tokens/:id revokes token", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/auth/tokens/"+tokenID, nil, authHeaders(token))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("DELETE /api/auth/tokens/:id not found", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/auth/tokens/00000000-0000-0000-0000-000000000000", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "API token not found")
	})
}
