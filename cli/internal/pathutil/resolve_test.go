package pathutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docshare/cli/internal/api"
)

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"00000000-0000-0000-0000-000000000000", true},
		{"ABCDEF01-2345-6789-ABCD-EF0123456789", true},
		{"not-a-uuid", false},
		{"550e8400e29b41d4a716446655440000", false},
		{"", false},
		{"550e8400-e29b-41d4-a716-44665544000", false},
		{"550e8400-e29b-41d4-a716-4466554400000", false},
		{"550e8400xe29bx41d4xa716x446655440000", false},
		{"gggggggg-gggg-gggg-gggg-gggggggggggg", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isUUID(tt.input)
			if got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	t.Run("empty path returns empty", func(t *testing.T) {
		id, err := Resolve(nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "" {
			t.Errorf("expected empty id, got %q", id)
		}
	})

	t.Run("root path returns empty", func(t *testing.T) {
		id, err := Resolve(nil, "/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "" {
			t.Errorf("expected empty id, got %q", id)
		}
	})

	t.Run("dot path returns empty", func(t *testing.T) {
		id, err := Resolve(nil, ".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "" {
			t.Errorf("expected empty id, got %q", id)
		}
	})

	t.Run("UUID passthrough", func(t *testing.T) {
		uuid := "550e8400-e29b-41d4-a716-446655440000"
		id, err := Resolve(nil, uuid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != uuid {
			t.Errorf("expected %q, got %q", uuid, id)
		}
	})

	t.Run("resolves path via API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/files" {
				_ = json.NewEncoder(w).Encode(api.Response[[]api.File]{
					Success: true,
					Data: []api.File{
						{ID: "dir-123", Name: "Documents", IsDirectory: true},
					},
				})
				return
			}
			if r.URL.Path == "/files/dir-123/children" {
				_ = json.NewEncoder(w).Encode(api.Response[[]api.File]{
					Success: true,
					Data: []api.File{
						{ID: "file-456", Name: "report.pdf", IsDirectory: false},
					},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := api.NewClient(server.URL, "test-token")
		id, err := Resolve(client, "/Documents/report.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "file-456" {
			t.Errorf("expected file-456, got %q", id)
		}
	})

	t.Run("not found in root returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(api.Response[[]api.File]{
				Success: true,
				Data:    []api.File{},
			})
		}))
		defer server.Close()

		client := api.NewClient(server.URL, "test-token")
		_, err := Resolve(client, "/NonExistent")
		if err == nil {
			t.Fatal("expected error for non-existent path")
		}
	})

	t.Run("not found in subdirectory returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/files" {
				_ = json.NewEncoder(w).Encode(api.Response[[]api.File]{
					Success: true,
					Data: []api.File{
						{ID: "dir-123", Name: "Docs", IsDirectory: true},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(api.Response[[]api.File]{
				Success: true,
				Data:    []api.File{},
			})
		}))
		defer server.Close()

		client := api.NewClient(server.URL, "test-token")
		_, err := Resolve(client, "/Docs/Missing")
		if err == nil {
			t.Fatal("expected error for missing nested path")
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(api.Response[[]api.File]{
				Success: true,
				Data: []api.File{
					{ID: "dir-abc", Name: "Documents", IsDirectory: true},
				},
			})
		}))
		defer server.Close()

		client := api.NewClient(server.URL, "test-token")
		id, err := Resolve(client, "/documents")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "dir-abc" {
			t.Errorf("expected dir-abc, got %q", id)
		}
	})

	t.Run("whitespace path is trimmed", func(t *testing.T) {
		id, err := Resolve(nil, "  /  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "" {
			t.Errorf("expected empty, got %q", id)
		}
	})
}
