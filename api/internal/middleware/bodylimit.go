package middleware

import (
	"strings"

	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

// SmallBodyLimitForNonUploadRoutes returns a middleware that rejects requests
// whose declared Content-Length exceeds maxBytes, *unless* the request is
// hitting one of the upload endpoints that legitimately accepts large bodies
// (the legacy multipart `/api/files/upload` and the chunked
// `/api/transfers/:code/upload`).
//
// We keep Fiber's global `BodyLimit` large enough to accept the legacy
// multipart upload (governed by MAX_UPLOAD_MB), but the rest of the API
// — auth, JSON CRUD, presign/finalize — shouldn't accept gigabyte JSON
// payloads. This middleware narrows the DoS surface without per-route
// fasthttp tuning, which Fiber doesn't expose. It only blocks honest
// (Content-Length-declaring) clients; chunked-encoded requests still hit
// the global cap.
func SmallBodyLimitForNonUploadRoutes(maxBytes int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isLargeBodyRoute(c.Path()) {
			return c.Next()
		}
		if length := c.Request().Header.ContentLength(); length > maxBytes {
			return utils.Error(c, fiber.StatusRequestEntityTooLarge, "request body too large")
		}
		return c.Next()
	}
}

func isLargeBodyRoute(path string) bool {
	// Exact match — must NOT match /api/files/upload/presign or
	// /api/files/upload/finalize (those are small JSON requests).
	if path == "/api/files/upload" {
		return true
	}
	// Transfer chunk uploads: /api/transfers/{code}/upload
	if strings.HasPrefix(path, "/api/transfers/") && strings.HasSuffix(path, "/upload") {
		return true
	}
	return false
}
