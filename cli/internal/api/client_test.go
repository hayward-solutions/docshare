package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	t.Run("creates client with correct base URL", func(t *testing.T) {
		client := NewClient("http://localhost:8080/", "test-token")
		if client.BaseURL != "http://localhost:8080/api" {
			t.Errorf("expected BaseURL 'http://localhost:8080/api', got %s", client.BaseURL)
		}
		if client.Token != "test-token" {
			t.Errorf("expected Token 'test-token', got %s", client.Token)
		}
	})

	t.Run("removes trailing slash from base URL", func(t *testing.T) {
		client := NewClient("http://example.com///", "")
		if client.BaseURL != "http://example.com/api" {
			t.Errorf("expected BaseURL 'http://example.com/api', got %s", client.BaseURL)
		}
	})

	t.Run("sets default HTTP client timeout", func(t *testing.T) {
		client := NewClient("http://localhost:8080", "")
		if client.HTTPClient == nil {
			t.Error("expected HTTPClient to be set")
		}
		if client.HTTPClient.Timeout == 0 {
			t.Error("expected HTTPClient to have a timeout set")
		}
	})
}

func TestAPIError(t *testing.T) {
	t.Run("formats error message correctly", func(t *testing.T) {
		err := &APIError{Status: 404, Message: "not found"}
		expected := "api: 404 — not found"
		if err.Error() != expected {
			t.Errorf("expected error message %q, got %q", expected, err.Error())
		}
	})
}

func TestClient_Get(t *testing.T) {
	t.Run("makes GET request with correct headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET request, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("expected Authorization header 'Bearer test-token', got %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("expected Accept header 'application/json', got %s", r.Header.Get("Accept"))
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-token")
		var result map[string]string
		if err := client.Get("/test", nil, &result); err != nil {
			t.Fatalf("Get() returned error: %v", err)
		}
	})

	t.Run("appends query parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") != "1" {
				t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
			}
			if r.URL.Query().Get("limit") != "10" {
				t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		params := map[string][]string{"page": {"1"}, "limit": {"10"}}
		var result map[string]string
		if err := client.Get("/test", params, &result); err != nil {
			t.Fatalf("Get() returned error: %v", err)
		}
	})

	t.Run("returns APIError on non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		var result map[string]string
		err := client.Get("/test", nil, &result)
		if err == nil {
			t.Fatal("expected error for 404 status")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected APIError, got %T", err)
		}
		if apiErr.Status != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", apiErr.Status)
		}
	})
}

func TestClient_Post(t *testing.T) {
	t.Run("makes POST request with JSON body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST request, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
			}

			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("failed to decode request body: %v", err)
			}
			if body["name"] != "test" {
				t.Errorf("expected name 'test', got %s", body["name"])
			}

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "123"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		var result map[string]string
		if err := client.Post("/test", map[string]string{"name": "test"}, &result); err != nil {
			t.Fatalf("Post() returned error: %v", err)
		}
		if result["id"] != "123" {
			t.Errorf("expected id '123', got %s", result["id"])
		}
	})

	t.Run("handles nil body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		if err := client.Post("/test", nil, nil); err != nil {
			t.Fatalf("Post() returned error: %v", err)
		}
	})
}

func TestClient_Put(t *testing.T) {
	t.Run("makes PUT request with JSON body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT request, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"updated": "true"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		var result map[string]string
		if err := client.Put("/test", map[string]string{"name": "updated"}, &result); err != nil {
			t.Fatalf("Put() returned error: %v", err)
		}
	})
}

func TestClient_Delete(t *testing.T) {
	t.Run("makes DELETE request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE request, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"deleted": "true"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		var result map[string]string
		if err := client.Delete("/test", &result); err != nil {
			t.Fatalf("Delete() returned error: %v", err)
		}
	})
}

func TestClient_PostForm(t *testing.T) {
	t.Run("makes POST request with form data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST request, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("expected Content-Type 'application/x-www-form-urlencoded', got %s", r.Header.Get("Content-Type"))
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("failed to parse form: %v", err)
			}
			if r.FormValue("grant_type") != "device_code" {
				t.Errorf("expected grant_type 'device_code', got %s", r.FormValue("grant_type"))
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "token123"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		formData := map[string][]string{"grant_type": {"device_code"}}
		var result map[string]string
		if err := client.PostForm("/oauth/token", formData, &result); err != nil {
			t.Fatalf("PostForm() returned error: %v", err)
		}
	})
}

func TestClient_Upload(t *testing.T) {
	t.Run("uploads file with multipart form", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.txt")
		content := []byte("test file content")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST request, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") == "" {
				t.Error("expected Content-Type header to be set")
			}
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Errorf("failed to parse multipart form: %v", err)
			}
			if r.FormValue("parentID") != "folder-123" {
				t.Errorf("expected parentID 'folder-123', got %s", r.FormValue("parentID"))
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("failed to get uploaded file: %v", err)
			}
			defer file.Close()
			if header.Filename != "test.txt" {
				t.Errorf("expected filename 'test.txt', got %s", header.Filename)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "file-123"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		extraFields := map[string]string{"parentID": "folder-123"}
		var result map[string]string
		if err := client.Upload("/files/upload", "file", filePath, extraFields, &result); err != nil {
			t.Fatalf("Upload() returned error: %v", err)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		client := NewClient("http://localhost:8080", "")
		err := client.Upload("/upload", "file", "/nonexistent/file.txt", nil, nil)
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}

func TestClient_DownloadToFile(t *testing.T) {
	t.Run("downloads file to disk", func(t *testing.T) {
		content := []byte("downloaded content")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET request, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		destPath := filepath.Join(tempDir, "downloaded.txt")

		client := NewClient(server.URL, "")
		if err := client.DownloadToFile(server.URL+"/download", destPath); err != nil {
			t.Fatalf("DownloadToFile() returned error: %v", err)
		}

		data, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("failed to read downloaded file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("expected content %q, got %q", string(content), string(data))
		}
	})

	t.Run("returns APIError on non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()

		tempDir := t.TempDir()
		destPath := filepath.Join(tempDir, "downloaded.txt")

		client := NewClient(server.URL, "")
		err := client.DownloadToFile(server.URL+"/download", destPath)
		if err == nil {
			t.Error("expected error for 404 status")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected APIError, got %T", err)
		}
		if apiErr.Status != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", apiErr.Status)
		}
	})
}

func TestResponse_Envelope(t *testing.T) {
	t.Run("parses success response with pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]map[string]string]{
				Success: true,
				Data: []map[string]string{
					{"id": "1", "name": "file1"},
					{"id": "2", "name": "file2"},
				},
				Pagination: &Pagination{
					Page:       1,
					Limit:      20,
					Total:      2,
					TotalPages: 1,
				},
			})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		var result Response[[]map[string]string]
		if err := client.Get("/files", nil, &result); err != nil {
			t.Fatalf("Get() returned error: %v", err)
		}

		if !result.Success {
			t.Error("expected Success to be true")
		}
		if len(result.Data) != 2 {
			t.Errorf("expected 2 items, got %d", len(result.Data))
		}
		if result.Pagination == nil {
			t.Fatal("expected Pagination to be set")
		}
		if result.Pagination.Total != 2 {
			t.Errorf("expected Total 2, got %d", result.Pagination.Total)
		}
	})

	t.Run("parses error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"success": "false",
				"error":   "invalid request",
			})
		}))
		defer server.Close()

		client := NewClient(server.URL, "")
		var result Response[any]
		err := client.Get("/test", nil, &result)
		if err == nil {
			t.Fatal("expected error for 400 status")
		}
	})
}
