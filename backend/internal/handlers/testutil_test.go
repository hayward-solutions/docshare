package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docshare/backend/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func setupTestApp() *fiber.App {
	app := fiber.New()

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")
	api.Get("/version", GetVersion)

	authHandler := NewAuthHandler(nil, nil)
	authMiddleware := middleware.NewAuthMiddleware(nil)

	authRoutes := api.Group("/auth")
	authRoutes.Post("/register", authHandler.Register)
	authRoutes.Post("/login", authHandler.Login)
	authRoutes.Get("/me", authMiddleware.RequireAuth, authHandler.Me)

	api.Get("/protected", authMiddleware.RequireAuth, func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})

	return app
}

func performRequest(t *testing.T, app *fiber.App, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}

	return resp
}

func performJSONRequest(t *testing.T, app *fiber.App, method, path string, payload any, headers map[string]string) *http.Response {
	t.Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal json payload for %s %s: %v", method, path, err)
		}
		body = bytes.NewReader(encoded)
	}

	requestHeaders := map[string]string{}
	for key, value := range headers {
		requestHeaders[key] = value
	}
	if payload != nil {
		requestHeaders["Content-Type"] = "application/json"
	}

	return performRequest(t, app, method, path, body, requestHeaders)
}

func decodeJSONMap(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading response body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("failed decoding response body as json: %v; body=%q", err, string(raw))
	}

	return payload
}
