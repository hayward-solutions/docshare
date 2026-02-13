package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestUsersEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	admin, adminToken := createTestUser(t, env.db, "users-admin@test.com", "password123", models.UserRoleAdmin)
	member, memberToken := createTestUser(t, env.db, "users-member@test.com", "password123", models.UserRoleUser)

	t.Run("GET /api/users/search returns users for authenticated request", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/search?search=users-", nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		if success, _ := body["success"].(bool); !success {
			t.Fatalf("expected success=true")
		}
	})

	t.Run("GET /api/users/ admin list users", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/?page=1&limit=2", nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		if _, ok := body["pagination"].(map[string]any); !ok {
			t.Fatalf("expected pagination object in list response")
		}
	})

	t.Run("GET /api/users/ non-admin is forbidden", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/", nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "admin access required")
	})

	t.Run("GET /api/users/:id returns user for admin", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, fmt.Sprintf("/api/users/%s", member.ID), nil, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("GET /api/users/:id not found", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/users/00000000-0000-0000-0000-000000000000", nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "user not found")
	})

	t.Run("PUT /api/users/:id admin update user", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{
			"firstName": "ChangedByAdmin",
		}, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].(map[string]any)
		if data["firstName"] != "ChangedByAdmin" {
			t.Fatalf("expected updated firstName")
		}
	})

	t.Run("PUT /api/users/:id empty firstName returns bad request", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/users/%s", member.ID), map[string]any{
			"firstName": "",
		}, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "firstName cannot be empty")
	})

	t.Run("DELETE /api/users/:id admin delete user", func(t *testing.T) {
		victim, _ := createTestUser(t, env.db, "users-delete-victim@test.com", "password123", models.UserRoleUser)
		resp := performRequest(t, env.app, http.MethodDelete, fmt.Sprintf("/api/users/%s", victim.ID), nil, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("DELETE /api/users/:id not found", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/users/00000000-0000-0000-0000-000000000000", nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "user not found")
	})

	if admin.ID == member.ID {
		t.Fatalf("expected distinct users for users endpoint tests")
	}
}
