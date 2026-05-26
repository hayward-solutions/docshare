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

	t.Run("rejects key not prefixed with caller user ID", func(t *testing.T) {
		// Use a UUID that is not the owner's ID
		foreignPrefix := "00000000-0000-0000-0000-000000000000"
		if foreignPrefix == owner.ID.String() {
			foreignPrefix = "11111111-1111-1111-1111-111111111111"
		}
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  foreignPrefix + "/abc/x.txt",
			"name": "x.txt",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "key does not belong to authenticated user")
	})

	t.Run("rejects invalid filename", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/upload/finalize", map[string]any{
			"key":  owner.ID.String() + "/abc/x.txt",
			"name": "",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid filename")
	})
}
