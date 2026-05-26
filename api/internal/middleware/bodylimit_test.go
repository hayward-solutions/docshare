package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSmallBodyLimitForNonUploadRoutes(t *testing.T) {
	app := fiber.New()
	app.Use(SmallBodyLimitForNonUploadRoutes(1024))
	app.Post("/api/auth/login", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/api/files/upload", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/api/files/upload/presign", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/api/transfers/abc123/upload", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Post("/api/transfers/a/b/upload", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Delete("/api/files/some-id", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	cases := []struct {
		name       string
		method     string
		path       string
		bodySize   int
		wantStatus int
	}{
		{"small body to JSON route passes", http.MethodPost, "/api/auth/login", 256, http.StatusOK},
		{"oversize body to JSON route is rejected", http.MethodPost, "/api/auth/login", 4096, http.StatusRequestEntityTooLarge},
		{"oversize body to presign is rejected", http.MethodPost, "/api/files/upload/presign", 4096, http.StatusRequestEntityTooLarge},
		{"oversize body to legacy multipart upload is allowed", http.MethodPost, "/api/files/upload", 4096, http.StatusOK},
		{"oversize body to transfer chunk upload is allowed", http.MethodPost, "/api/transfers/abc123/upload", 4096, http.StatusOK},
		{"oversize body to non-canonical transfer path is rejected", http.MethodPost, "/api/transfers/a/b/upload", 4096, http.StatusRequestEntityTooLarge},
		{"oversize DELETE body is rejected", http.MethodDelete, "/api/files/some-id", 4096, http.StatusRequestEntityTooLarge},
	}
	// Chunked-encoding rejection (when Content-Length is absent and
	// fasthttp reports ContentLength() == -1) is exercised in production
	// traffic; httptest + fasthttp's serializer don't compose cleanly
	// enough for a focused unit test here.

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.NewReader([]byte(strings.Repeat("a", tc.bodySize)))
			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, 5000)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, resp.StatusCode)
			}
		})
	}
}
