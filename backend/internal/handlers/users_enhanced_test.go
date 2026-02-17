package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestUsersEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	admin, adminToken := createTestUser(t, env.db, "users-enh-admin@test.com", "password123", models.UserRoleAdmin)
	member, _ := createTestUser(t, env.db, "users-enh-member@test.com", "password123", models.UserRoleUser)

	t.Run("GET /api/users/ with search filter", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/?search=enh-admin", nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatal("expected at least one user matching search")
		}
	})

	t.Run("GET /api/users/:id invalid UUID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/invalid-uuid", nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid user id")
	})

	t.Run("PUT /api/users/:id empty lastName", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{
			"lastName": "",
		}, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "lastName cannot be empty")
	})

	t.Run("PUT /api/users/:id update role to admin", func(t *testing.T) {
		targetUser, _ := createTestUser(t, env.db, "users-enh-role-target@test.com", "password123", models.UserRoleUser)
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", targetUser.ID), map[string]any{
			"role": "admin",
		}, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
		body := decodeJSONMap(t, resp)
		data := body["data"].(map[string]any)
		if data["role"] != "admin" {
			t.Fatalf("expected role admin, got %v", data["role"])
		}
	})

	t.Run("PUT /api/users/:id invalid role", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{
			"role": "superadmin",
		}, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid role")
	})

	t.Run("PUT /api/users/:id no fields to update", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{}, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "no valid fields to update")
	})

	t.Run("PUT /api/users/:id not found", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/users/00000000-0000-0000-0000-000000000000", map[string]any{
			"firstName": "Test",
		}, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "user not found")
	})

	t.Run("PUT /api/users/:id update avatarURL", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{
			"avatarURL": "https://example.com/photo.png",
		}, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
		body := decodeJSONMap(t, resp)
		data := body["data"].(map[string]any)
		if data["avatarURL"] != "https://example.com/photo.png" {
			t.Fatalf("expected updated avatarURL")
		}
	})

	t.Run("PUT /api/users/:id clear avatarURL", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{
			"avatarURL": "",
		}, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("GET /api/users/search with limit parameter", func(t *testing.T) {
		_, memberToken := createTestUser(t, env.db, "users-search-limit@test.com", "password123", models.UserRoleUser)
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/search?search=users&limit=1", nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) > 1 {
			t.Fatalf("expected at most 1 result, got %d", len(data))
		}
	})

	t.Run("GET /api/users/search requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/search?search=test", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("DELETE /api/users/:id invalid UUID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/users/invalid-uuid", nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid user id")
	})

	_ = admin
}
