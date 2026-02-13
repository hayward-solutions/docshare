package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupResponseTestApp() *fiber.App {
	app := fiber.New()

	app.Get("/success", func(c *fiber.Ctx) error {
		return Success(c, fiber.StatusCreated, fiber.Map{"id": "123"})
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		return Error(c, fiber.StatusBadRequest, "invalid input")
	})

	app.Get("/paginated", func(c *fiber.Ctx) error {
		return Paginated(c, []string{"a", "b"}, 2, 20, 45)
	})

	return app
}

func performResponseTestRequest(t *testing.T, app *fiber.App, path string) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request to %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed decoding %s response body: %v", path, err)
	}

	body["_statusCode"] = float64(resp.StatusCode)
	return body
}

func requireNumberField(t *testing.T, obj map[string]any, key string) int {
	t.Helper()

	raw, ok := obj[key]
	if !ok {
		t.Fatalf("expected field %q to exist in response", key)
	}

	number, ok := raw.(float64)
	if !ok {
		t.Fatalf("expected field %q to be numeric, got %T", key, raw)
	}

	return int(number)
}

func TestResponseHelpers(t *testing.T) {
	app := setupResponseTestApp()

	t.Run("Success returns expected envelope", func(t *testing.T) {
		body := performResponseTestRequest(t, app, "/success")

		if status := requireNumberField(t, body, "_statusCode"); status != fiber.StatusCreated {
			t.Fatalf("expected status %d, got %d", fiber.StatusCreated, status)
		}

		success, ok := body["success"].(bool)
		if !ok || !success {
			t.Fatalf("expected success=true, got %v", body["success"])
		}

		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data object, got %T", body["data"])
		}
		if data["id"] != "123" {
			t.Fatalf("expected data.id to be %q, got %v", "123", data["id"])
		}
	})

	t.Run("Error returns expected envelope", func(t *testing.T) {
		body := performResponseTestRequest(t, app, "/error")

		if status := requireNumberField(t, body, "_statusCode"); status != fiber.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", fiber.StatusBadRequest, status)
		}

		success, ok := body["success"].(bool)
		if !ok || success {
			t.Fatalf("expected success=false, got %v", body["success"])
		}
		if body["error"] != "invalid input" {
			t.Fatalf("expected error message %q, got %v", "invalid input", body["error"])
		}
	})

	t.Run("Paginated returns data and pagination metadata", func(t *testing.T) {
		body := performResponseTestRequest(t, app, "/paginated")

		if status := requireNumberField(t, body, "_statusCode"); status != fiber.StatusOK {
			t.Fatalf("expected status %d, got %d", fiber.StatusOK, status)
		}

		success, ok := body["success"].(bool)
		if !ok || !success {
			t.Fatalf("expected success=true, got %v", body["success"])
		}

		data, ok := body["data"].([]any)
		if !ok {
			t.Fatalf("expected data array, got %T", body["data"])
		}
		if len(data) != 2 {
			t.Fatalf("expected data length 2, got %d", len(data))
		}

		pagination, ok := body["pagination"].(map[string]any)
		if !ok {
			t.Fatalf("expected pagination object, got %T", body["pagination"])
		}

		if page := requireNumberField(t, pagination, "page"); page != 2 {
			t.Fatalf("expected page=2, got %d", page)
		}
		if limit := requireNumberField(t, pagination, "limit"); limit != 20 {
			t.Fatalf("expected limit=20, got %d", limit)
		}
		if total := requireNumberField(t, pagination, "total"); total != 45 {
			t.Fatalf("expected total=45, got %d", total)
		}
		if totalPages := requireNumberField(t, pagination, "totalPages"); totalPages != 3 {
			t.Fatalf("expected totalPages=3, got %d", totalPages)
		}
	})
}
