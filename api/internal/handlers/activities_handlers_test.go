package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestActivitiesEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	user, token := createTestUser(t, env.db, "activities-user@test.com", "password123", models.UserRoleUser)
	actor, _ := createTestUser(t, env.db, "activities-actor@test.com", "password123", models.UserRoleUser)

	activity1 := models.Activity{
		UserID:       user.ID,
		ActorID:      actor.ID,
		Action:       "file.upload",
		ResourceType: "file",
		ResourceName: "Report",
		Message:      "uploaded report",
		IsRead:       false,
	}
	activity2 := models.Activity{
		UserID:       user.ID,
		ActorID:      actor.ID,
		Action:       "share.create",
		ResourceType: "file",
		ResourceName: "Budget",
		Message:      "shared budget",
		IsRead:       false,
	}
	if err := env.db.Create(&activity1).Error; err != nil {
		t.Fatalf("failed creating activity1: %v", err)
	}
	if err := env.db.Create(&activity2).Error; err != nil {
		t.Fatalf("failed creating activity2: %v", err)
	}

	t.Run("GET /api/activities/ list paginated activities", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/activities/?page=1&limit=1", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		if _, ok := body["pagination"].(map[string]any); !ok {
			t.Fatalf("expected pagination info")
		}
	})

	t.Run("GET /api/activities/unread-count returns count", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/activities/unread-count", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		count := int(body["data"].(map[string]any)["count"].(float64))
		if count < 2 {
			t.Fatalf("expected unread count >= 2, got %d", count)
		}
	})

	t.Run("PUT /api/activities/:id/read marks activity as read", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPut, fmt.Sprintf("/api/activities/%s/read", activity1.ID), nil, authHeaders(token))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("PUT /api/activities/:id/read not found", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPut, "/api/activities/00000000-0000-0000-0000-000000000000/read", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "activity not found")
	})

	t.Run("PUT /api/activities/read-all marks all as read", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPut, "/api/activities/read-all", nil, authHeaders(token))
		assertStatus(t, resp, http.StatusOK)

		var unread int64
		env.db.Model(&models.Activity{}).Where("user_id = ? AND is_read = false", user.ID).Count(&unread)
		if unread != 0 {
			t.Fatalf("expected no unread activities after read-all, got %d", unread)
		}
	})
}
