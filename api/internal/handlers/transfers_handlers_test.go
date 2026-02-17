package handlers

import (
	"net/http"
	"testing"

	"github.com/docshare/api/internal/models"
)

func TestTransfersEndpoints(t *testing.T) {
	env := setupTestEnv(t)
	_, senderToken := createTestUser(t, env.db, "transfer-sender@test.com", "password123", models.UserRoleUser)
	_, recipientToken := createTestUser(t, env.db, "transfer-recipient@test.com", "password123", models.UserRoleUser)

	t.Run("POST /api/transfers create transfer", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "test-file.txt",
			"fileSize": 1024,
		}, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusCreated)

		data := body["data"].(map[string]any)
		if data["code"] == nil || data["code"] == "" {
			t.Fatal("expected non-empty code")
		}
		if data["fileName"] != "test-file.txt" {
			t.Errorf("expected fileName test-file.txt, got %v", data["fileName"])
		}
	})

	t.Run("POST /api/transfers missing fileName", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileSize": 1024,
		}, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "fileName is required")
	})

	t.Run("POST /api/transfers missing fileSize", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "test.txt",
		}, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "fileSize must be positive")
	})

	t.Run("POST /api/transfers invalid fileSize", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "test.txt",
			"fileSize": -1,
		}, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "fileSize must be positive")
	})

	t.Run("GET /api/transfers list own transfers", func(t *testing.T) {
		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "to-list.txt",
			"fileSize": 100,
		}, authHeaders(senderToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers", nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].([]any)
		if len(data) == 0 {
			t.Fatal("expected at least one transfer")
		}
	})

	t.Run("GET /api/transfers unauthorized", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers", nil, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/transfers/:code poll pending transfer", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "poll-test.txt",
			"fileSize": 512,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code, nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "pending" {
			t.Errorf("expected status pending, got %v", data["status"])
		}
	})

	t.Run("GET /api/transfers/:code poll non-existent", func(t *testing.T) {
		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/NONEXIST", nil, authHeaders(senderToken))
		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("POST /api/transfers/:code/connect recipient connects", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "connect-test.txt",
			"fileSize": 256,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "connected" {
			t.Errorf("expected status connected, got %v", data["status"])
		}
	})

	t.Run("POST /api/transfers/:code/connect sender cannot connect to own", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "self-connect.txt",
			"fileSize": 256,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusBadRequest)
		assertEnvelopeError(t, body, "cannot connect to your own transfer")
	})

	t.Run("POST /api/transfers/:code/connect non-existent", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/NONEXIST/connect", nil, authHeaders(recipientToken))
		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("POST /api/transfers/:code/complete after connection", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "complete-test.txt",
			"fileSize": 128,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/complete", nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "completed" {
			t.Errorf("expected status completed, got %v", data["status"])
		}
	})

	t.Run("DELETE /api/transfers/:code sender cancels", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "cancel-test.txt",
			"fileSize": 64,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performRequest(t, env.app, http.MethodDelete, "/api/transfers/"+code, nil, authHeaders(senderToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "cancelled" {
			t.Errorf("expected status cancelled, got %v", data["status"])
		}
	})

	t.Run("DELETE /api/transfers/:code non-sender cannot cancel", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "unauthorized-cancel.txt",
			"fileSize": 64,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		resp := performRequest(t, env.app, http.MethodDelete, "/api/transfers/"+code, nil, authHeaders(recipientToken))
		assertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("GET /api/transfers/:code after completion shows gone", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "already-done.txt",
			"fileSize": 32,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))
		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/complete", nil, authHeaders(senderToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code, nil, authHeaders(senderToken))
		assertStatus(t, resp, http.StatusGone)
	})

	t.Run("POST /api/transfers unauthorized", func(t *testing.T) {
		resp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "test.txt",
			"fileSize": 100,
		}, nil)
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /api/transfers/:code recipient can poll after connect", func(t *testing.T) {
		createResp := performJSONRequest(t, env.app, http.MethodPost, "/api/transfers", map[string]any{
			"fileName": "recipient-poll.txt",
			"fileSize": 200,
		}, authHeaders(senderToken))
		createBody := decodeJSONMap(t, createResp)
		code := createBody["data"].(map[string]any)["code"].(string)

		performJSONRequest(t, env.app, http.MethodPost, "/api/transfers/"+code+"/connect", nil, authHeaders(recipientToken))

		resp := performRequest(t, env.app, http.MethodGet, "/api/transfers/"+code, nil, authHeaders(recipientToken))
		body := decodeJSONMap(t, resp)
		assertStatus(t, resp, http.StatusOK)

		data := body["data"].(map[string]any)
		if data["status"] != "active" && data["status"] != "receiver_connected" {
			t.Errorf("expected status active or receiver_connected, got %v", data["status"])
		}
	})
}
