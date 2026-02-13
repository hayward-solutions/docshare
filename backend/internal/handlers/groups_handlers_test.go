package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestGroupsEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "groups-owner@test.com", "password123", models.UserRoleUser)
	member, memberToken := createTestUser(t, env.db, "groups-member@test.com", "password123", models.UserRoleUser)
	adminUser, adminToken := createTestUser(t, env.db, "groups-admin@test.com", "password123", models.UserRoleUser)

	var groupID string

	t.Run("POST /api/groups/ create group and owner membership", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/", map[string]any{
			"name": "Team Alpha",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)
		data := body["data"].(map[string]any)
		groupID = data["id"].(string)

		var membership models.GroupMembership
		err := env.db.First(&membership, "group_id = ? AND user_id = ?", groupID, owner.ID).Error
		if err != nil {
			t.Fatalf("expected owner membership to exist: %v", err)
		}
		if membership.Role != models.GroupRoleOwner {
			t.Fatalf("expected owner role, got %s", membership.Role)
		}
	})

	t.Run("GET /api/groups/ lists only memberships", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected at least one group in owner list")
		}
	})

	t.Run("GET /api/groups/:id non-member forbidden", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/"+groupID, nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "group access denied")
	})

	t.Run("GET /api/groups/:id member can fetch details", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/"+groupID, nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("POST /api/groups/:id/members add member", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": member.ID.String(),
			"role":   "member",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/groups/:id/members invalid role", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": adminUser.ID.String(),
			"role":   "invalid-role",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid role")
	})

	t.Run("PUT /api/groups/:id update owner allowed", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/groups/"+groupID, map[string]any{
			"name": "Team Alpha Updated",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("PUT /api/groups/:id update member forbidden", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/groups/"+groupID, map[string]any{
			"name": "Cannot Update",
		}, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "insufficient permissions")
	})

	t.Run("PUT /api/groups/:id/members/:userId update role", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/groups/%s/members/%s", groupID, member.ID), map[string]any{
			"role": "admin",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("DELETE /api/groups/:id/members/:userId cannot remove owner", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, fmt.Sprintf("/api/groups/%s/members/%s", groupID, owner.ID), nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "cannot remove group owner")
	})

	t.Run("DELETE /api/groups/:id delete admin forbidden", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/groups/"+groupID, nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "only group owner can delete the group")
	})

	t.Run("DELETE /api/groups/:id delete owner allowed", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/groups/"+groupID, nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	_ = adminToken
}
