package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/backend/internal/models"
)

func TestPublicFileEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	owner, _ := createTestUser(t, env.db, "public-owner@test.com", "password123", models.UserRoleUser)
	recipient, recipientToken := createTestUser(t, env.db, "public-recipient@test.com", "password123", models.UserRoleUser)

	t.Run("GET /api/public/files/:id", func(t *testing.T) {
		t.Run("returns file for unauthenticated user with public share", func(t *testing.T) {
			file := models.File{
				Name:        "public-file.txt",
				MimeType:    "text/plain",
				Size:        100,
				IsDirectory: false,
				OwnerID:     owner.ID,
				StoragePath: "public-file.txt",
			}
			if err := env.db.Create(&file).Error; err != nil {
				t.Fatalf("failed creating file: %v", err)
			}

			share := models.Share{
				FileID:     file.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicAnyone,
				Permission: models.SharePermissionView,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+file.ID.String(), nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)
			data := body["data"].(map[string]any)
			if data["name"] != "public-file.txt" {
				t.Errorf("expected file name 'public-file.txt', got %v", data["name"])
			}
		})

		t.Run("returns file for authenticated user with private share", func(t *testing.T) {
			file := models.File{
				Name:        "private-shared-file.txt",
				MimeType:    "text/plain",
				Size:        50,
				IsDirectory: false,
				OwnerID:     owner.ID,
				StoragePath: "private-shared-file.txt",
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

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+file.ID.String(), nil, authHeaders(recipientToken))
			assertStatus(t, resp, http.StatusOK)
		})

		t.Run("returns not found for non-existent file", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/00000000-0000-0000-0000-000000000000", nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusNotFound)
			assertEnvelopeError(t, body, "file not found")
		})

		t.Run("returns unauthorized for public_logged_in without auth", func(t *testing.T) {
			file := models.File{
				Name:        "logged-in-only.txt",
				MimeType:    "text/plain",
				Size:        25,
				IsDirectory: false,
				OwnerID:     owner.ID,
				StoragePath: "logged-in-only.txt",
			}
			if err := env.db.Create(&file).Error; err != nil {
				t.Fatalf("failed creating file: %v", err)
			}

			share := models.Share{
				FileID:     file.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicLoggedIn,
				Permission: models.SharePermissionView,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+file.ID.String(), nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusUnauthorized)
			assertEnvelopeError(t, body, "login required to access this file")
		})

		t.Run("returns file for authenticated user with public_logged_in share", func(t *testing.T) {
			file := models.File{
				Name:        "logged-in-accessible.txt",
				MimeType:    "text/plain",
				Size:        30,
				IsDirectory: false,
				OwnerID:     owner.ID,
				StoragePath: "logged-in-accessible.txt",
			}
			if err := env.db.Create(&file).Error; err != nil {
				t.Fatalf("failed creating file: %v", err)
			}

			share := models.Share{
				FileID:     file.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicLoggedIn,
				Permission: models.SharePermissionView,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+file.ID.String(), nil, authHeaders(recipientToken))
			assertStatus(t, resp, http.StatusOK)
		})
	})

	t.Run("GET /api/public/files/:id/download", func(t *testing.T) {
		t.Run("returns not found for non-existent file", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/00000000-0000-0000-0000-000000000000/download", nil, nil)
			assertStatus(t, resp, http.StatusNotFound)
		})

		t.Run("returns unauthorized for public_logged_in without auth", func(t *testing.T) {
			file := models.File{
				Name:        "download-logged-in.txt",
				MimeType:    "text/plain",
				Size:        20,
				IsDirectory: false,
				OwnerID:     owner.ID,
				StoragePath: "download-logged-in.txt",
			}
			if err := env.db.Create(&file).Error; err != nil {
				t.Fatalf("failed creating file: %v", err)
			}

			share := models.Share{
				FileID:     file.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicLoggedIn,
				Permission: models.SharePermissionDownload,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+file.ID.String()+"/download", nil, nil)
			assertStatus(t, resp, http.StatusUnauthorized)
		})
	})

	t.Run("GET /api/public/files/:id/children", func(t *testing.T) {
		t.Run("returns not found for non-existent directory", func(t *testing.T) {
			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/00000000-0000-0000-0000-000000000000/children", nil, nil)
			assertStatus(t, resp, http.StatusNotFound)
		})

		t.Run("returns bad request for non-directory", func(t *testing.T) {
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

			share := models.Share{
				FileID:     file.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicAnyone,
				Permission: models.SharePermissionView,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+file.ID.String()+"/children", nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusBadRequest)
			assertEnvelopeError(t, body, "file is not a directory")
		})

		t.Run("returns children for public directory", func(t *testing.T) {
			parentDir := models.File{
				Name:        "public-folder",
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
				Name:        "child-file.txt",
				MimeType:    "text/plain",
				Size:        15,
				IsDirectory: false,
				ParentID:    &parentDir.ID,
				OwnerID:     owner.ID,
				StoragePath: "child-file.txt",
			}
			if err := env.db.Create(&childFile).Error; err != nil {
				t.Fatalf("failed creating child: %v", err)
			}

			share := models.Share{
				FileID:     parentDir.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicAnyone,
				Permission: models.SharePermissionView,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+parentDir.ID.String()+"/children", nil, nil)
			body := decodeJSONMap(t, resp)
			assertStatus(t, resp, http.StatusOK)

			data := body["data"].([]any)
			if len(data) != 1 {
				t.Errorf("expected 1 child, got %d", len(data))
			}
		})

		t.Run("returns unauthorized for public_logged_in without auth", func(t *testing.T) {
			parentDir := models.File{
				Name:        "logged-in-folder",
				MimeType:    "inode/directory",
				Size:        0,
				IsDirectory: true,
				OwnerID:     owner.ID,
				StoragePath: "",
			}
			if err := env.db.Create(&parentDir).Error; err != nil {
				t.Fatalf("failed creating parent: %v", err)
			}

			share := models.Share{
				FileID:     parentDir.ID,
				SharedByID: owner.ID,
				ShareType:  models.ShareTypePublicLoggedIn,
				Permission: models.SharePermissionView,
			}
			if err := env.db.Create(&share).Error; err != nil {
				t.Fatalf("failed creating share: %v", err)
			}

			resp := performRequest(t, env.app, http.MethodGet, "/api/public/files/"+parentDir.ID.String()+"/children", nil, nil)
			assertStatus(t, resp, http.StatusUnauthorized)
		})
	})
}
