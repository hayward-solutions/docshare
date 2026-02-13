package pathutil

import (
	"fmt"
	"strings"

	"github.com/docshare/cli/internal/api"
)

// Resolve converts a human-readable path (e.g. "/Documents/Reports") to the UUID of the
// final segment by walking the API directory tree from root. An empty or "/" path means root.
// A valid UUID is returned as-is (passthrough).
func Resolve(client *api.Client, path string) (string, error) {
	path = strings.TrimSpace(path)

	// Empty or root — caller should handle listing root.
	if path == "" || path == "/" || path == "." {
		return "", nil
	}

	// If it looks like a UUID already, return it directly.
	if isUUID(path) {
		return path, nil
	}

	// Remove leading/trailing slashes and split.
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	currentID := "" // empty = root

	for _, segment := range parts {
		if segment == "" {
			continue
		}

		children, err := listChildren(client, currentID)
		if err != nil {
			return "", fmt.Errorf("listing %q: %w", segment, err)
		}

		found := false
		for _, f := range children {
			if strings.EqualFold(f.Name, segment) {
				currentID = f.ID
				found = true
				break
			}
		}
		if !found {
			if currentID == "" {
				return "", fmt.Errorf("not found in root: %s", segment)
			}
			return "", fmt.Errorf("not found: %s", segment)
		}
	}

	return currentID, nil
}

func listChildren(client *api.Client, parentID string) ([]api.File, error) {
	var resp api.Response[[]api.File]
	var err error
	if parentID == "" {
		err = client.Get("/files", nil, &resp)
	} else {
		err = client.Get("/files/"+parentID+"/children", nil, &resp)
	}
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("api error: %s", resp.Error)
	}
	return resp.Data, nil
}

func isUUID(s string) bool {
	// Quick check: 36 chars, with hyphens at positions 8, 13, 18, 23.
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
