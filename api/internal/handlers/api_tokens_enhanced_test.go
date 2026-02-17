package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestAPITokenEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	_, token := createTestUser(t, env.db, "api-token-enh@test.com", "password123", models.UserRoleUser)

	t.Run("POST /api/auth/tokens/ name too long", func(t *testing.T) {
		longName := strings.Repeat("a", 256)
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name": longName,
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "name must be 255 characters or less")
	})

	t.Run("POST /api/auth/tokens/ with never expiry", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name":      "Never Token",
			"expiresIn": "never",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)

		data := body["data"].(map[string]any)
		apiToken := data["apiToken"].(map[string]any)
		if apiToken["expiresAt"] != nil {
			t.Fatalf("expected null expiresAt for never expiry, got %v", apiToken["expiresAt"])
		}
	})

	t.Run("POST /api/auth/tokens/ with 90d expiry", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name":      "90 Day Token",
			"expiresIn": "90d",
		}, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)

		data := body["data"].(map[string]any)
		rawToken := data["token"].(string)
		if !strings.HasPrefix(rawToken, "dsh_") {
			t.Fatalf("expected token to start with dsh_, got %s", rawToken[:4])
		}
	})

	t.Run("POST /api/auth/tokens/ with 365d expiry", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name":      "365 Day Token",
			"expiresIn": "365d",
		}, authHeaders(token))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/auth/tokens/ requires authentication", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/auth/tokens/", map[string]any{
			"name": "No Auth Token",
		}, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/auth/tokens/ requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/tokens/", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("DELETE /api/auth/tokens/:id invalid UUID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/auth/tokens/invalid-uuid", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid token ID")
	})

	t.Run("GET /api/auth/tokens/ includes pagination", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/auth/tokens/?page=1&limit=5", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		if _, ok := body["pagination"]; !ok {
			t.Fatal("expected pagination in response")
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("isValidSharePermission", func(t *testing.T) {
		tests := []struct {
			input string
			want  bool
		}{
			{"view", true},
			{"download", true},
			{"edit", true},
			{"VIEW", true},
			{"  edit  ", true},
			{"invalid", false},
			{"", false},
			{"admin", false},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				if got := isValidSharePermission(tt.input); got != tt.want {
					t.Errorf("isValidSharePermission(%q) = %v, want %v", tt.input, got, tt.want)
				}
			})
		}
	})

	t.Run("isValidShareType", func(t *testing.T) {
		tests := []struct {
			input string
			want  bool
		}{
			{"private", true},
			{"public_anyone", true},
			{"public_logged_in", true},
			{"PRIVATE", true},
			{"  public_anyone  ", true},
			{"invalid", false},
			{"", false},
			{"public", false},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				if got := isValidShareType(tt.input); got != tt.want {
					t.Errorf("isValidShareType(%q) = %v, want %v", tt.input, got, tt.want)
				}
			})
		}
	})

	t.Run("parseUUID valid", func(t *testing.T) {
		id, err := parseUUID("00000000-0000-0000-0000-000000000001")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if id.String() != "00000000-0000-0000-0000-000000000001" {
			t.Errorf("expected parsed UUID, got %s", id.String())
		}
	})

	t.Run("parseUUID with whitespace", func(t *testing.T) {
		id, err := parseUUID("  00000000-0000-0000-0000-000000000001  ")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if id.String() != "00000000-0000-0000-0000-000000000001" {
			t.Errorf("expected parsed UUID, got %s", id.String())
		}
	})

	t.Run("parseUUID invalid", func(t *testing.T) {
		_, err := parseUUID("not-a-uuid")
		if err == nil {
			t.Fatal("expected error for invalid UUID")
		}
	})
}
