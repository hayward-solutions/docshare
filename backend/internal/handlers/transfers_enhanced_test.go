package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/docshare/backend/internal/models"
)

func TestTransfersEndpoints_Enhanced(t *testing.T) {
	env := setupTestEnv(t)
	_, senderToken := createTestUser(t, env.db, "transfer-enh-sender@test.com", "password123", models.UserRoleUser)
	_, recipientToken := createTestUser(t, env.db, "transfer-enh-recipient@test.com", "password123", models.UserRoleUser)
	_, outsiderToken := createTestUser(t, env.db, "transfer-enh-outsider@test.com", "password123", models.UserRoleUser)

	t.Run("POST /api/transfers with custom timeout", func(t *testing.T) {
		timeout := 600
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "timeout-test.txt",
			"fileSize": 512,
			"timeout":  timeout,
		}, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)

		data := body["data"].(map[string]any)
		if data["code"] == nil || data["code"] == "" {
			t.Fatal("expected non-empty code")
		}
	})

	t.Run("POST /api/transfers requires authentication", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "test.txt",
			"fileSize": 100,
		}, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/transfers/:code outsider cannot poll", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "outsider-poll.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code, nil, authHeaders(outsiderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "not authorized for this transfer")
	})

	t.Run("POST /api/transfers/:code/connect already connected returns conflict", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "double-connect.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(outsiderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusConflict)
		assertEnvelopeError(t, body, "transfer is not pending")
	})

	t.Run("POST /api/transfers/:code/upload not sender returns forbidden", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "upload-forbidden.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/upload", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "not the sender")
	})

	t.Run("POST /api/transfers/:code/upload before connection returns bad request", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "upload-no-connect.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/upload", nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "receiver not connected")
	})

	t.Run("GET /api/transfers/:code/download not recipient returns forbidden", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "download-forbidden.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code+"/download", nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "not the recipient")
	})

	t.Run("GET /api/transfers/:code/download before active returns bad request", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "download-inactive.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))
		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/complete", nil, authHeaders(senderToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code+"/download", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "transfer not active")
	})

	t.Run("POST /api/transfers/:code/complete unauthorized user returns forbidden", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "complete-forbidden.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/complete", nil, authHeaders(outsiderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusForbidden)
		assertEnvelopeError(t, body, "not authorized")
	})

	t.Run("DELETE /api/transfers/:code completed transfer returns bad request", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "cancel-completed.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))
		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/complete", nil, authHeaders(senderToken))

		resp := performRequest(t, env.app, http.MethodDelete, "/api/transfers/"+code, nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "transfer already completed")
	})

	t.Run("GET /api/transfers/:code cancelled transfer returns gone", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "cancelled-poll.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performRequest(t, env.app, http.MethodDelete, "/api/transfers/"+code, nil, authHeaders(senderToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code, nil, authHeaders(senderToken))
		assertStatus(t, resp, http.StatusGone)
	})

	t.Run("POST /api/transfers/:code/connect expired transfer returns gone", func(t *testing.T) {
		sender, _ := createTestUser(t, env.db, "transfer-expired-sender@test.com", "password123", models.UserRoleUser)
		transfer := models.Transfer{
			Code:      "EXPIRE",
			SenderID:  sender.ID,
			FileName:  "expired.txt",
			FileSize:  100,
			Status:    models.TransferStatusPending,
			Timeout:   1,
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		env.db.Create(&transfer)

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/EXPIRE/connect", nil, authHeaders(recipientToken))
		assertStatus(t, resp, http.StatusGone)
	})

	t.Run("GET /api/transfers/:code sender sees receiver_connected after connect", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "sender-poll-active.txt",
			"fileSize": 200,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code, nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "receiver_connected" {
			t.Errorf("expected status receiver_connected, got %v", data["status"])
		}
	})

	t.Run("POST /api/transfers/:code/complete recipient can complete", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "recipient-complete.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/complete", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "completed" {
			t.Errorf("expected status completed, got %v", data["status"])
		}
	})
}

func TestCleanupExpiredTransfers(t *testing.T) {
	env := setupTestEnv(t)
	sender, _ := createTestUser(t, env.db, "cleanup-sender@test.com", "password123", models.UserRoleUser)

	expired := models.Transfer{
		Code:      "CLEAN1",
		SenderID:  sender.ID,
		FileName:  "expired.txt",
		FileSize:  100,
		Status:    models.TransferStatusPending,
		Timeout:   1,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	env.db.Create(&expired)

	active := models.Transfer{
		Code:      "CLEAN2",
		SenderID:  sender.ID,
		FileName:  "active.txt",
		FileSize:  100,
		Status:    models.TransferStatusPending,
		Timeout:   300,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	env.db.Create(&active)

	CleanupExpiredTransfers(env.db)

	var expiredTransfer models.Transfer
	env.db.First(&expiredTransfer, "code = ?", "CLEAN1")
	if expiredTransfer.Status != models.TransferStatusExpired {
		t.Errorf("expected expired status, got %s", expiredTransfer.Status)
	}

	var activeTransfer models.Transfer
	env.db.First(&activeTransfer, "code = ?", "CLEAN2")
	if activeTransfer.Status != models.TransferStatusPending {
		t.Errorf("expected pending status, got %s", activeTransfer.Status)
	}
}
