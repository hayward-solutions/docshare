package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
	"github.com/google/uuid"
)

func TestGroupsEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	_, ownerToken := createTestUser(t, env.db, "groups-enhanced-owner@test.com", "password123", models.UserRoleUser)
	member, memberToken := createTestUser(t, env.db, "groups-enhanced-member@test.com", "password123", models.UserRoleUser)
	outsider, outsiderToken := createTestUser(t, env.db, "groups-enhanced-outsider@test.com", "password123", models.UserRoleUser)

	var groupID string

	t.Run("POST /api/groups/ empty name", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/", map[string]any{
			"name": "",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "name is required")
	})

	t.Run("POST /api/groups/ success", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/", map[string]any{
			"name": "Enhanced Test Group",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)
		data := body["data"].(map[string]any)
		groupID = data["id"].(string)
	})

	t.Run("GET /api/groups/:id outsider forbidden", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/"+groupID, nil, authHeaders(outsiderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "group access denied")
	})

	t.Run("GET /api/groups/:id invalid UUID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/invalid-uuid", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid group id")
	})

	t.Run("GET /api/groups/:id non-member", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/00000000-0000-0000-0000-000000000000", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "group access denied")
	})

	t.Run("POST /api/groups/:id/members add member success", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": member.ID.String(),
			"role":   "member",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/groups/:id/members add duplicate member", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": member.ID.String(),
			"role":   "member",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusConflict)
		assertEnvelopeError(t, body, "user is already a member")
	})

	t.Run("POST /api/groups/:id/members non-admin forbidden", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": outsider.ID.String(),
			"role":   "member",
		}, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "insufficient permissions")
	})

	t.Run("POST /api/groups/:id/members non-existent user", func(t *testing.T) {
		fakeID := uuid.New()
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": fakeID.String(),
			"role":   "member",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "user not found")
	})

	t.Run("PUT /api/groups/:id empty name", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/groups/"+groupID, map[string]any{
			"name": "",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "name cannot be empty")
	})

	t.Run("PUT /api/groups/:id success by owner", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/groups/"+groupID, map[string]any{
			"name": "Owner Updated Name",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("DELETE /api/groups/:id/members/:userId member cannot remove others", func(t *testing.T) {
		anotherUser, _ := createTestUser(t, env.db, "groups-yet-another@test.com", "password123", models.UserRoleUser)

		performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": anotherUser.ID.String(),
			"role":   "member",
		}, authHeaders(ownerToken))

		resp := performRequest(t, env.app, http.MethodDelete, "/api/groups/"+groupID+"/members/"+anotherUser.ID.String(), nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "insufficient permissions")
	})

	t.Run("DELETE /api/groups/:id only owner can delete", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/groups/"+groupID, nil, authHeaders(memberToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "only group owner can delete the group")
	})

	t.Run("DELETE /api/groups/:id success", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/groups/"+groupID, nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	_ = outsiderToken
}

func TestGroupsEndpoints_AdminScenarios(t *testing.T) {
	env := setupTestEnv(t)
	_, ownerToken := createTestUser(t, env.db, "groups-admin-owner@test.com", "password123", models.UserRoleUser)
	adminUser, adminToken := createTestUser(t, env.db, "groups-admin@test.com", "password123", models.UserRoleUser)
	regularMember, _ := createTestUser(t, env.db, "groups-regular@test.com", "password123", models.UserRoleUser)

	t.Run("setup group with admin", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/", map[string]any{
			"name": "Admin Test Group",
		}, authHeaders(ownerToken))
		_ = decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)
	})

	var groupID string
	t.Run("get group id", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/groups/", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		data := body["data"].([]any)
		groupMap := data[0].(map[string]any)
		groupID = groupMap["id"].(string)
	})

	t.Run("add admin member", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": adminUser.ID.String(),
			"role":   "admin",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("PUT /api/groups/:id admin can update", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/groups/"+groupID, map[string]any{
			"name": "Admin Updated Name",
		}, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("add regular member", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": regularMember.ID.String(),
			"role":   "member",
		}, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("PUT /api/groups/:id/members/:userId admin can update role to member", func(t *testing.T) {
		anotherMember, _ := createTestUser(t, env.db, "groups-another-member@test.com", "password123", models.UserRoleUser)

		performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": anotherMember.ID.String(),
			"role":   "admin",
		}, authHeaders(ownerToken))

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/groups/"+groupID+"/members/"+anotherMember.ID.String(), map[string]any{
			"role": "member",
		}, authHeaders(adminToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("DELETE /api/groups/:id/members/:userId admin cannot remove other admin", func(t *testing.T) {
		anotherAdmin, _ := createTestUser(t, env.db, "groups-another-admin@test.com", "password123", models.UserRoleUser)

		performJSONRequest(t, env.app, http.MethodPost, "/api/groups/"+groupID+"/members", map[string]any{
			"userID": anotherAdmin.ID.String(),
			"role":   "admin",
		}, authHeaders(ownerToken))

		resp := performRequest(t, env.app, http.MethodDelete, "/api/groups/"+groupID+"/members/"+anotherAdmin.ID.String(), nil, authHeaders(adminToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "admins cannot remove other admins")
	})
}
