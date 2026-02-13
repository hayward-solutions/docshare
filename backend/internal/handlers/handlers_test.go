package handlers

import (
	"net/http"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	resp := performRequest(t, env.app, http.MethodGet, "/health", nil, nil)
	body := decodeJSONMap(t, resp)

	assertStatus(t, resp, http.StatusOK)
	if got, _ := body["status"].(string); got != "ok" {
		t.Fatalf("expected health status 'ok', got %q", got)
	}
}

func TestVersionEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	resp := performRequest(t, env.app, http.MethodGet, "/api/version", nil, nil)
	body := decodeJSONMap(t, resp)

	assertStatus(t, resp, http.StatusOK)
	if success, _ := body["success"].(bool); !success {
		t.Fatalf("expected success=true, got %+v", body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %T", body["data"])
	}
	if data["apiVersion"] != "v1" {
		t.Fatalf("expected apiVersion v1, got %v", data["apiVersion"])
	}
}
