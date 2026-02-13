package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func parseUUID(value string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(value))
}

func isValidSharePermission(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "view", "download", "edit":
		return true
	default:
		return false
	}
}

func isValidShareType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "private", "public_anyone", "public_logged_in":
		return true
	default:
		return false
	}
}

func getRequestID(c *fiber.Ctx) string {
	if rid, ok := c.Locals("requestID").(string); ok {
		return rid
	}
	return ""
}
