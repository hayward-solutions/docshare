package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/docshare/backend/internal/models"
)

func TestAuditExportEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	user, token := createTestUser(t, env.db, "audit-user@test.com", "password123", models.UserRoleUser)

	entry := models.AuditLog{
		UserID:       &user.ID,
		Action:       "file.upload",
		ResourceType: "file",
		Details: map[string]any{
			"file_name": "report.pdf",
		},
		IPAddress: "127.0.0.1",
		CreatedAt: time.Now().UTC(),
	}
	if err := env.db.Create(&entry).Error; err != nil {
		t.Fatalf("failed creating audit log fixture: %v", err)
	}

	t.Run("GET /api/audit-log/export?format=json", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/audit-log/export?format=json", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)
		if body["success"] != true {
			t.Fatalf("expected success=true JSON export")
		}
	})

	t.Run("GET /api/audit-log/export?format=csv", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/audit-log/export?format=csv", nil, authHeaders(token))
		assertStatus(t, resp, http.StatusOK)
		if contentType := resp.Header.Get("Content-Type"); contentType == "" {
			t.Fatalf("expected content-type header for csv export")
		}
	})

	t.Run("GET /api/audit-log/export?format=invalid", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/audit-log/export?format=invalid", nil, authHeaders(token))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "format must be csv or json")
	})
}
