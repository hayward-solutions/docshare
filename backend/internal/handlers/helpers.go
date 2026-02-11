package handlers

import (
	"strings"

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
