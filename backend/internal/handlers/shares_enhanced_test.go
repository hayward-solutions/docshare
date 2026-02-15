package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/docshare/backend/internal/models"
	"github.com/google/uuid"
)

func TestSharesEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "shares-enhanced-owner@test.com", "password123", models.UserRoleUser)
	recipient, recipientToken := createTestUser(t, env.db, "shares-enhanced-recipient@test.com", "password123", models.UserRoleUser)
	thirdUser, thirdToken := createTestUser(t, env.db, "shares-enhanced-third@test.com", "password123", models.UserRoleUser)

	t.Run("POST /api/files/:id/share non-owner forbidden", func(t *testing.T) {
		file := models.File{
			Name:        "non-owner-share.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "non-owner-share.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"userID":     thirdUser.ID.String(),
			"permission": "view",
		}, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "insufficient permissions")
	})

	t.Run("POST /api/files/:id/share with non-existent user", func(t *testing.T) {
		file := models.File{
			Name:        "share-missing-user.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "share-missing-user.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		fakeUserID := uuid.New()
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"userID":     fakeUserID.String(),
			"permission": "view",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "target user not found")
	})

	t.Run("POST /api/files/:id/share with expired date", func(t *testing.T) {
		file := models.File{
			Name:        "share-expired.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "share-expired.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		pastTime := time.Now().Add(-24 * time.Hour)
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"userID":     recipient.ID.String(),
			"permission": "view",
			"expiresAt":  pastTime.Format(time.RFC3339),
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusCreated)
	})

	t.Run("POST /api/files/:id/share duplicate public share", func(t *testing.T) {
		file := models.File{
			Name:        "duplicate-public.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "duplicate-public.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"shareType":  "public_anyone",
			"permission": "view",
		}, authHeaders(ownerToken))

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"shareType":  "public_anyone",
			"permission": "download",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusConflict)
		assertEnvelopeError(t, body, "a public share of this type already exists for this file")
	})

	t.Run("POST /api/files/:id/share invalid share type", func(t *testing.T) {
		file := models.File{
			Name:        "invalid-share-type.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "invalid-share-type.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"shareType":  "invalid_type",
			"permission": "view",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid share type")
	})

	t.Run("POST /api/files/:id/share public with userID error", func(t *testing.T) {
		file := models.File{
			Name:        "public-with-user.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "public-with-user.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"shareType":  "public_anyone",
			"userID":     recipient.ID.String(),
			"permission": "view",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "userID and groupID must not be set for public shares")
	})

	t.Run("POST /api/files/:id/share private without userID or groupID", func(t *testing.T) {
		file := models.File{
			Name:        "private-no-target.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "private-no-target.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/"+file.ID.String()+"/share", map[string]any{
			"permission": "view",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "exactly one of userID or groupID is required for private shares")
	})

	t.Run("PUT /api/shares/:id non-owner forbidden", func(t *testing.T) {
		file := models.File{
			Name:        "share-update-forbidden.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "share-update-forbidden.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		share := models.Share{
			FileID:           file.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &recipient.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
		}
		if err := env.db.Create(&share).Error; err != nil {
			t.Fatalf("failed creating share: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/shares/"+share.ID.String(), map[string]any{
			"permission": "edit",
		}, authHeaders(thirdToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "insufficient permissions")
	})

	t.Run("DELETE /api/shares/:id non-owner forbidden", func(t *testing.T) {
		file := models.File{
			Name:        "share-delete-forbidden.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "share-delete-forbidden.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		share := models.Share{
			FileID:           file.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &recipient.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
		}
		if err := env.db.Create(&share).Error; err != nil {
			t.Fatalf("failed creating share: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodDelete, "/api/shares/"+share.ID.String(), nil, authHeaders(thirdToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "insufficient permissions")
	})

	t.Run("GET /api/shared with search filter", func(t *testing.T) {
		file := models.File{
			Name:        "shared-search-test.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "shared-search-test.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		share := models.Share{
			FileID:           file.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &recipient.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
		}
		if err := env.db.Create(&share).Error; err != nil {
			t.Fatalf("failed creating share: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/shared?search=search", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatal("expected at least one shared file")
		}
	})

	t.Run("GET /api/shared includes pagination", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/shared?page=1&limit=10", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		if _, ok := body["pagination"]; !ok {
			t.Fatal("expected pagination in response")
		}
	})

	_ = thirdToken
}
