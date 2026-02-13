package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/backend/internal/models"
	"github.com/google/uuid"
)

func TestFilesEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "files-owner@test.com", "password123", models.UserRoleUser)
	otherUser, otherToken := createTestUser(t, env.db, "files-other@test.com", "password123", models.UserRoleUser)

	var rootDirID string
	var nestedDirID string

	t.Run("POST /api/files/directory create root directory", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/directory", map[string]any{
			"name": "Documents",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)
		rootDirID = body["data"].(map[string]any)["id"].(string)
	})

	t.Run("POST /api/files/directory create nested directory", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/directory", map[string]any{
			"name":     "Subfolder",
			"parentID": rootDirID,
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)
		nestedDirID = body["data"].(map[string]any)["id"].(string)
	})

	t.Run("POST /api/files/directory missing name", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/files/directory", map[string]any{}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "name is required")
	})

	t.Run("GET /api/files/ list root files", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected root files to include created directory")
		}
	})

	t.Run("GET /api/files/:id returns file", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+rootDirID, nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("GET /api/files/:id not found", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/00000000-0000-0000-0000-000000000000", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "file not found")
	})

	t.Run("GET /api/files/:id access denied", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+rootDirID, nil, authHeaders(otherToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "access denied")
	})

	t.Run("GET /api/files/:id/children lists children", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+rootDirID+"/children", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected nested directory in children list")
		}
	})

	t.Run("GET /api/files/search query too short", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/search?q=a", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "search query must be at least 2 characters")
	})

	t.Run("GET /api/files/search returns results", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/search?q=Sub", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatalf("expected search results")
		}
	})

	t.Run("GET /api/files/:id/path returns breadcrumb", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+nestedDirID+"/path", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].([]any)
		if len(data) != 2 {
			t.Fatalf("expected 2 breadcrumb entries, got %d", len(data))
		}
	})

	t.Run("PUT /api/files/:id rename success", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/files/"+nestedDirID, map[string]any{
			"name": "SubfolderRenamed",
		}, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("PUT /api/files/:id empty name", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/files/"+nestedDirID, map[string]any{
			"name": "",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "name cannot be empty")
	})

	t.Run("PUT /api/files/:id cannot be parent of itself", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/files/"+nestedDirID, map[string]any{
			"parentID": nestedDirID,
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "file cannot be parent of itself")
	})

	t.Run("GET /api/files/:id/download-url returns URL for owned file", func(t *testing.T) {
		file := models.File{
			Name:        "report.txt",
			MimeType:    "text/plain",
			Size:        1,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "owner/path/report.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file for download-url test: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/download-url", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		data := body["data"].(map[string]any)
		if data["url"] != "/api/files/"+file.ID.String()+"/download" {
			t.Fatalf("unexpected download URL %v", data["url"])
		}
	})

	t.Run("DELETE /api/files/:id delete directory", func(t *testing.T) {
		toDelete := models.File{
			Name:        "DeleteMeDir",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			OwnerID:     owner.ID,
			StoragePath: "",
		}
		if err := env.db.Create(&toDelete).Error; err != nil {
			t.Fatalf("failed creating directory for delete test: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodDelete, "/api/files/"+toDelete.ID.String(), nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)

		var count int64
		env.db.Model(&models.File{}).Where("id = ?", toDelete.ID).Count(&count)
		if count != 0 {
			t.Fatalf("expected deleted directory to be removed")
		}
	})

	if owner.ID == uuid.Nil || otherUser.ID == uuid.Nil {
		t.Fatalf("expected users to have UUID IDs")
	}
	if rootDirID == "" || nestedDirID == "" {
		t.Fatalf("expected directory IDs to be set")
	}

}
