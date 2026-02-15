package handlers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestFilesEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "files-enhanced-owner@test.com", "password123", models.UserRoleUser)
	otherUser, otherToken := createTestUser(t, env.db, "files-enhanced-other@test.com", "password123", models.UserRoleUser)

	t.Run("POST /api/files/upload requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPost, "/api/files/upload", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("POST /api/files/upload requires file", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodPost, "/api/files/upload", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "file is required")
	})

	t.Run("POST /api/files/upload invalid parentID", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("parentID", "invalid-uuid")
		part, _ := writer.CreateFormFile("file", "test.txt")
		_, _ = io.WriteString(part, "content")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+ownerToken)

		resp, err := env.app.Test(req, 10000)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("POST /api/files/upload parentID not found", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("parentID", "00000000-0000-0000-0000-000000000000")
		part, _ := writer.CreateFormFile("file", "test.txt")
		_, _ = io.WriteString(part, "content")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+ownerToken)

		resp, err := env.app.Test(req, 10000)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/files/ requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/files/:id invalid UUID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/invalid-uuid", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid file id")
	})

	t.Run("GET /api/files/:id/children for non-directory", func(t *testing.T) {
		file := models.File{
			Name:        "not-a-dir.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "not-a-dir.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/children", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "file is not a directory")
	})

	t.Run("GET /api/files/:id/download for directory", func(t *testing.T) {
		dir := models.File{
			Name:        "download-dir",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			OwnerID:     owner.ID,
			StoragePath: "",
		}
		if err := env.db.Create(&dir).Error; err != nil {
			t.Fatalf("failed creating directory: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+dir.ID.String()+"/download", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "cannot download a directory")
	})

	t.Run("GET /api/files/search with directoryID filter", func(t *testing.T) {
		parentDir := models.File{
			Name:        "search-parent",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			OwnerID:     owner.ID,
			StoragePath: "",
		}
		if err := env.db.Create(&parentDir).Error; err != nil {
			t.Fatalf("failed creating parent: %v", err)
		}

		childFile := models.File{
			Name:        "search-child.txt",
			MimeType:    "text/plain",
			Size:        50,
			IsDirectory: false,
			ParentID:    &parentDir.ID,
			OwnerID:     owner.ID,
			StoragePath: "search-child.txt",
		}
		if err := env.db.Create(&childFile).Error; err != nil {
			t.Fatalf("failed creating child: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/search?q=search&directoryID="+parentDir.ID.String(), nil, authHeaders(ownerToken))
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("GET /api/files/search with invalid directoryID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/search?q=test&directoryID=invalid-uuid", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid directoryID")
	})

	t.Run("GET /api/files/search with non-existent directoryID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/files/search?q=test&directoryID=00000000-0000-0000-0000-000000000000", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "directory not found")
	})

	t.Run("PUT /api/files/:id move to non-existent parent", func(t *testing.T) {
		file := models.File{
			Name:        "move-test.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "move-test.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/files/"+file.ID.String(), map[string]any{
			"parentID": "00000000-0000-0000-0000-000000000000",
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusNotFound)
		assertEnvelopeError(t, body, "new parent not found")
	})

	t.Run("PUT /api/files/:id move to file instead of directory", func(t *testing.T) {
		file := models.File{
			Name:        "file-to-move.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "file-to-move.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		notDir := models.File{
			Name:        "not-a-dir-parent.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "not-a-dir-parent.txt",
		}
		if err := env.db.Create(&notDir).Error; err != nil {
			t.Fatalf("failed creating notDir: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/files/"+file.ID.String(), map[string]any{
			"parentID": notDir.ID.String(),
		}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "new parent must be a directory")
	})

	t.Run("PUT /api/files/:id no valid fields to update", func(t *testing.T) {
		file := models.File{
			Name:        "no-update.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "no-update.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performJSONRequest(t, env.app, http.MethodPut, "/api/files/"+file.ID.String(), map[string]any{}, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "no valid fields to update")
	})

	t.Run("DELETE /api/files/:id requires authentication", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/files/00000000-0000-0000-0000-000000000000", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("DELETE /api/files/:id invalid UUID", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodDelete, "/api/files/invalid-uuid", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "invalid file id")
	})

	t.Run("DELETE /api/files/:id access denied", func(t *testing.T) {
		file := models.File{
			Name:        "delete-denied.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "delete-denied.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodDelete, "/api/files/"+file.ID.String(), nil, authHeaders(otherToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "access denied")
	})

	t.Run("GET /api/files/:id/path for nested file", func(t *testing.T) {
		rootDir := models.File{
			Name:        "path-root",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			OwnerID:     owner.ID,
			StoragePath: "",
		}
		if err := env.db.Create(&rootDir).Error; err != nil {
			t.Fatalf("failed creating root dir: %v", err)
		}

		midDir := models.File{
			Name:        "path-mid",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			ParentID:    &rootDir.ID,
			OwnerID:     owner.ID,
			StoragePath: "",
		}
		if err := env.db.Create(&midDir).Error; err != nil {
			t.Fatalf("failed creating mid dir: %v", err)
		}

		leafFile := models.File{
			Name:        "path-leaf.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			ParentID:    &midDir.ID,
			OwnerID:     owner.ID,
			StoragePath: "path-leaf.txt",
		}
		if err := env.db.Create(&leafFile).Error; err != nil {
			t.Fatalf("failed creating leaf file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+leafFile.ID.String()+"/path", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].([]any)
		if len(data) != 3 {
			t.Errorf("expected 3 path entries, got %d", len(data))
		}
	})

	t.Run("GET /api/files/:id/path access denied", func(t *testing.T) {
		file := models.File{
			Name:        "path-denied.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "path-denied.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/path", nil, authHeaders(otherToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "access denied")
	})

	_ = otherUser
}
