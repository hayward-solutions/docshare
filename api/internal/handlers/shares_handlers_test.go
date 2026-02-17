package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
	"github.com/google/uuid"
)

func TestSharesEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "shares-owner@test.com", "password123", models.UserRoleUser)
	recipient, recipientToken := createTestUser(t, env.db, "shares-recipient@test.com", "password123", models.UserRoleUser)

	file := models.File{
		Name:        "shared-doc.txt",
		MimeType:    "text/plain",
		Size:        32,
		IsDirectory: false,
		OwnerID:     owner.ID,
		StoragePath: "owner/shared-doc.txt",
	}
	if err := env.db.Create(&file).Error; err != nil {
		t.Fatalf("failed creating file fixture: %v", err)
	}

	t.Run("POST /api/files/:id/share create private share with user", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"userID":     recipient.ID.String(),
			"permission": "view",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/files/:id/share invalid permission", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"userID":     recipient.ID.String(),
			"permission": "invalid",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid permission")
	})

	t.Run("POST /api/files/:id/share share with yourself", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"userID":     owner.ID.String(),
			"permission": "view",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "cannot share with yourself")
	})

	t.Run("POST /api/files/:id/share create public share", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"shareType":  "public_anyone",
			"permission": "download",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("GET /api/files/:id/shares list shares for file", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/shares", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) < 2 {
			t.Fatalf("expected at least 2 shares, got %d", len(data))
		}
	})

	t.Run("PUT /api/shares/:id update share permission", func(t *testing.T) {
		var share models.Share
		if err := env.db.Where("file_id = ? AND shared_with_user_id = ?", file.ID, recipient.ID).First(&share).Error; err != nil {
			t.Fatalf("failed loading share for update: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/shares/"+share.ID.String(), map[string]any{
			"permission": "edit",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("DELETE /api/shares/:id revoke share", func(t *testing.T) {
		share := models.Share{
			FileID:           file.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &recipient.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
		}
		if err := env.db.Create(&share).Error; err != nil {
			t.Fatalf("failed creating share for delete: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodDelete, "/api/shares/"+share.ID.String(), nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("GET /api/shared lists files shared with me", func(t *testing.T) {
		share := models.Share{
			FileID:           file.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &recipient.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
			ExpiresAt:        nil,
		}
		if err := env.db.Create(&share).Error; err != nil {
			t.Fatalf("failed creating share for shared-with-me: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/shared", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected shared files list to include file")
		}
	})

	if owner.ID == uuid.Nil {
		t.Fatalf("expected non-nil owner UUID")
	}
}
