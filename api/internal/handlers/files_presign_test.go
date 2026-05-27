package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestPresignUpload(t *testing.T) {
	env := setupTestEnv(t)
	_, ownerToken := createTestUser(t, env.db, "presign-owner@test.com", "password123", models.UserRoleUser)

	t.Run("requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", "not-json", authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid request body")
	})

	t.Run("rejects empty filename", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", map[string]any{
			"name": "",
			"size": 1024,
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid filename")
	})

	t.Run("rejects non-positive size", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", map[string]any{
			"name": "x.txt",
			"size": 0,
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "size must be positive")
	})

	t.Run("rejects size above MaxUploadBytes", func(t *testing.T) {
		// testutil_test.go configures MaxUploadBytes = 100MB
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", map[string]any{
			"name": "huge.bin",
			"size": int64(200) * 1024 * 1024,
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusRequestEntityTooLarge)
	})

	t.Run("rejects invalid parentID", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", map[string]any{
			"name":     "x.txt",
			"size":     1024,
			"parentID": "not-a-uuid",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid parentID")
	})

	t.Run("rejects missing parent folder", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/presign", map[string]any{
			"name":     "x.txt",
			"size":     1024,
			"parentID": "00000000-0000-0000-0000-000000000000",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "parent folder not found")
	})
}

func TestFinalizeUpload(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "finalize-owner@test.com", "password123", models.UserRoleUser)

	t.Run("requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", "not-json", authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid request body")
	})

	t.Run("rejects empty key", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  "",
			"name": "x.txt",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "key is required")
	})

	t.Run("rejects key without uploads/ staging prefix", func(t *testing.T) {
		// A key prefixed only with the caller's UUID (i.e. the legacy final
		// shape) must be rejected — finalize accepts staging keys only.
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  owner.ID.String() + "/abc/x.txt",
			"name": "x.txt",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "key does not belong to authenticated user")
	})

	t.Run("rejects staging key not prefixed with caller user ID", func(t *testing.T) {
		foreignID := "00000000-0000-0000-0000-000000000000"
		if foreignID == owner.ID.String() {
			foreignID = "11111111-1111-1111-1111-111111111111"
		}
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  "uploads/" + foreignID + "/abc/x.txt",
			"name": "x.txt",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "key does not belong to authenticated user")
	})

	t.Run("rejects invalid filename", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  "uploads/" + owner.ID.String() + "/abc/x.txt",
			"name": "",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid filename")
	})

	t.Run("rejects key with path traversal that escapes user prefix", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  "uploads/" + owner.ID.String() + "/../00000000-0000-0000-0000-000000000000/abc/x.txt",
			"name": "x.txt",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "key does not belong to authenticated user")
	})

	t.Run("rejects already-finalized key", func(t *testing.T) {
		// Storage paths in the DB are the FINAL form (without the uploads/
		// prefix). The finalize request supplies the matching STAGING key;
		// the handler derives the final key and looks it up.
		finalKey := owner.ID.String() + "/already-here/x.txt"
		seeded := models.File{
			Name:        "x.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: finalKey,
		}
		if err := env.db.Create(&seeded).Error; err != nil {
			t.Fatalf("failed seeding file row: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  "uploads/" + finalKey,
			"name": "x.txt",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusConflict)
		assertEnvelopeError(t, body, "upload already finalized")
	})
}
