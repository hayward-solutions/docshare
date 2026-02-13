package handlers

import (
	"net/http"
	"strings"
	"testing"
)

func assertErrorResponse(t *testing.T, statusCode int, body map[string]any, expectedStatus int, expectedMessage string) {
	t.Helper()

	if statusCode != expectedStatus {
		t.Fatalf("expected status code %d, got %d", expectedStatus, statusCode)
	}

	success, ok := body["success"].(bool)
	if !ok {
		t.Fatalf("expected success field to be boolean, got %T", body["success"])
	}
	if success {
		t.Fatalf("expected success=false, got %v", body["success"])
	}

	errMessage, ok := body["error"].(string)
	if !ok {
		t.Fatalf("expected error field to be string, got %T", body["error"])
	}
	if errMessage != expectedMessage {
		t.Fatalf("expected error message %q, got %q", expectedMessage, errMessage)
	}
}

func TestHealthEndpoint(t *testing.T) {
	app := setupTestApp()

	resp := performRequest(t, app, http.MethodGet, "/health", nil, nil)
	body := decodeJSONMap(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected health status %q, got %v", "ok", body["status"])
	}
}

func TestVersionEndpoint(t *testing.T) {
	app := setupTestApp()

	resp := performRequest(t, app, http.MethodGet, "/api/version", nil, nil)
	body := decodeJSONMap(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	success, ok := body["success"].(bool)
	if !ok || !success {
		t.Fatalf("expected success=true, got %v", body["success"])
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", body["data"])
	}

	if data["version"] != Version {
		t.Fatalf("expected version %q, got %v", Version, data["version"])
	}
	if data["apiVersion"] != "v1" {
		t.Fatalf("expected apiVersion %q, got %v", "v1", data["apiVersion"])
	}
}

func TestAuthValidationEndpoints(t *testing.T) {
	app := setupTestApp()

	t.Run("register rejects malformed json", func(t *testing.T) {
		resp := performRequest(t, app, http.MethodPost, "/api/auth/register", strings.NewReader("{"), map[string]string{
			"Content-Type": "application/json",
		})
		body := decodeJSONMap(t, resp)

		assertErrorResponse(t, resp.StatusCode, body, http.StatusBadRequest, "invalid request body")
	})

	t.Run("register rejects missing required fields", func(t *testing.T) {
		resp := performJSONRequest(t, app, http.MethodPost, "/api/auth/register", map[string]any{}, nil)
		body := decodeJSONMap(t, resp)

		assertErrorResponse(t, resp.StatusCode, body, http.StatusBadRequest, "invalid email")
	})

	t.Run("login rejects empty request body", func(t *testing.T) {
		resp := performJSONRequest(t, app, http.MethodPost, "/api/auth/login", map[string]any{}, nil)
		body := decodeJSONMap(t, resp)

		assertErrorResponse(t, resp.StatusCode, body, http.StatusBadRequest, "email and password are required")
	})

	t.Run("login rejects malformed json", func(t *testing.T) {
		resp := performRequest(t, app, http.MethodPost, "/api/auth/login", strings.NewReader("{"), map[string]string{
			"Content-Type": "application/json",
		})
		body := decodeJSONMap(t, resp)

		assertErrorResponse(t, resp.StatusCode, body, http.StatusBadRequest, "invalid request body")
	})
}

func TestAuthMiddlewareRejections(t *testing.T) {
	app := setupTestApp()

	testCases := []struct {
		name            string
		path            string
		authorization   string
		expectedStatus  int
		expectedMessage string
	}{
		{
			name:            "missing authorization header on protected endpoint",
			path:            "/api/protected",
			authorization:   "",
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "missing authorization header",
		},
		{
			name:            "missing authorization header on auth me endpoint",
			path:            "/api/auth/me",
			authorization:   "",
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "missing authorization header",
		},
		{
			name:            "malformed authorization header",
			path:            "/api/protected",
			authorization:   "Token abc",
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "invalid authorization format",
		},
		{
			name:            "bearer header without token value",
			path:            "/api/protected",
			authorization:   "Bearer ",
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "invalid authorization format",
		},
		{
			name:            "invalid jwt token",
			path:            "/api/protected",
			authorization:   "Bearer not-a-valid-jwt",
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "invalid or expired token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.authorization != "" {
				headers["Authorization"] = tc.authorization
			}

			resp := performRequest(t, app, http.MethodGet, tc.path, nil, headers)
			body := decodeJSONMap(t, resp)

			assertErrorResponse(t, resp.StatusCode, body, tc.expectedStatus, tc.expectedMessage)
		})
	}
}
