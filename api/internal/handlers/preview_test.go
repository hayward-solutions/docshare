package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestPreviewEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	owner, ownerToken := createTestUser(t, env.db, "preview-owner@test.com", "password123", models.UserRoleUser)
	_, otherToken := createTestUser(t, env.db, "preview-other@test.com", "password123", models.UserRoleUser)

	t.Run("GET /api/files/:id/preview returns preview URL", func(t *testing.T) {
		file := models.File{
			Name:        "preview-test.txt",
			MimeType:    "text/plain",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "preview-test.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/preview", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["path"] == nil || data["path"] == "" {
			t.Error("expected path in response")
		}
		if data["token"] == nil || data["token"] == "" {
			t.Error("expected token in response")
		}
	})

	t.Run("GET /api/files/:id/preview access denied for non-owner", func(t *testing.T) {
		file := models.File{
			Name:        "preview-denied.txt",
			MimeType:    "text/plain",
			Size:        50,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "preview-denied.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/preview", nil, authHeaders(otherToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "access denied")
	})

	t.Run("GET /api/files/:id/preview requires auth", func(t *testing.T) {
		file := models.File{
			Name:        "preview-no-auth.txt",
			MimeType:    "text/plain",
			Size:        25,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "preview-no-auth.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/preview", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/files/:id/preview-status returns job status", func(t *testing.T) {
		file := models.File{
			Name:        "status-test.pdf",
			MimeType:    "application/pdf",
			Size:        256,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "status-test.pdf",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		job := models.PreviewJob{
			FileID:      file.ID,
			Status:      "pending",
			MaxAttempts: 3,
			Attempts:    0,
		}
		if err := env.db.Create(&job).Error; err != nil {
			t.Fatalf("failed creating preview job: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/preview-status", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		jobResp := data["job"].(map[string]any)
		if jobResp["status"] != "pending" {
			t.Errorf("expected status 'pending', got %v", jobResp["status"])
		}
	})

	t.Run("GET /api/files/:id/preview-status returns nil job when none exists", func(t *testing.T) {
		file := models.File{
			Name:        "no-job-test.pdf",
			MimeType:    "application/pdf",
			Size:        128,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "no-job-test.pdf",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/preview-status", nil, authHeaders(ownerToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["job"] != nil {
			t.Errorf("expected nil job, got %v", data["job"])
		}
	})

	t.Run("GET /api/files/:id/preview-status access denied", func(t *testing.T) {
		file := models.File{
			Name:        "status-denied.pdf",
			MimeType:    "application/pdf",
			Size:        64,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "status-denied.pdf",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/preview-status", nil, authHeaders(otherToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "access denied")
	})

	t.Run("GET /api/files/:id/retry-preview access denied", func(t *testing.T) {
		file := models.File{
			Name:        "retry-denied.pdf",
			MimeType:    "application/pdf",
			Size:        32,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "retry-denied.pdf",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/retry-preview", nil, authHeaders(otherToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "access denied")
	})

	t.Run("GET /api/files/:id/proxy requires token or auth", func(t *testing.T) {
		file := models.File{
			Name:        "proxy-test.txt",
			MimeType:    "text/plain",
			Size:        10,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "proxy-test.txt",
		}
		if err := env.db.Create(&file).Error; err != nil {
			t.Fatalf("failed creating file: %v", err)
		}

		resp := performRequest(t, env.app, http.MethodGet, "/api/files/"+file.ID.String()+"/proxy", nil, nil)
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusUnauthorized)
		assertEnvelopeError(t, body, "unauthorized")
	})
}
