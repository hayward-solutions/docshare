package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"github.com/docshare/api/pkg/utils"
	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	logger.Init()
	utils.ConfigureJWT("middleware-test-secret", 24)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = db.AutoMigrate(&models.User{}, &models.APIToken{})
	if err != nil {
		t.Fatalf("failed automigrating: %v", err)
	}

	return db
}

func createMiddlewareTestUser(t *testing.T, db *gorm.DB, email string, role models.UserRole) (*models.User, string) {
	t.Helper()
	hash, _ := utils.HashPassword("password123")
	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		FirstName:    "Test",
		LastName:     "User",
		Role:         role,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed creating user: %v", err)
	}
	token, err := utils.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed generating token: %v", err)
	}
	return user, token
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("failed decoding body: %v body=%q", err, string(raw))
	}
	return body
}

func TestRequireAuth(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	auth := NewAuthMiddleware(db)
	_, token := createMiddlewareTestUser(t, db, "auth-require@test.com", models.UserRoleUser)

	app := fiber.New()
	app.Get("/protected", auth.RequireAuth, func(c *fiber.Ctx) error {
		user := GetCurrentUser(c)
		return c.JSON(fiber.Map{"email": user.Email})
	})

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
		if body["error"] != "missing authorization header" {
			t.Fatalf("expected missing header error, got %v", body["error"])
		}
	})

	t.Run("invalid authorization format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Basic somecreds")
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
		if body["error"] != "invalid authorization format" {
			t.Fatalf("expected invalid format error, got %v", body["error"])
		}
	})

	t.Run("invalid JWT token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-jwt-token")
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("valid JWT token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["email"] != "auth-require@test.com" {
			t.Fatalf("expected email to be auth-require@test.com, got %v", body["email"])
		}
	})

	t.Run("JWT for deleted user", func(t *testing.T) {
		deletedUser := &models.User{
			Email:        "deleted@test.com",
			PasswordHash: "hash",
			FirstName:    "Deleted",
			LastName:     "User",
			Role:         models.UserRoleUser,
		}
		db.Create(deletedUser)
		deletedToken, _ := utils.GenerateToken(deletedUser)
		db.Unscoped().Delete(deletedUser)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+deletedToken)
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestRequireAuth_APIToken(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	auth := NewAuthMiddleware(db)
	user, _ := createMiddlewareTestUser(t, db, "api-token-auth@test.com", models.UserRoleUser)

	rawToken := "dsh_abcdef1234567890abcdef1234567890abcdef12345678"
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	apiToken := models.APIToken{
		UserID:    user.ID,
		Name:      "test-token",
		TokenHash: tokenHash,
		Prefix:    rawToken[:8],
	}
	db.Create(&apiToken)

	app := fiber.New()
	app.Get("/protected", auth.RequireAuth, func(c *fiber.Ctx) error {
		u := GetCurrentUser(c)
		return c.JSON(fiber.Map{"email": u.Email})
	})

	t.Run("valid API token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+rawToken)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["email"] != "api-token-auth@test.com" {
			t.Fatalf("expected email, got %v", body["email"])
		}
	})

	t.Run("invalid API token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer dsh_invalidtokenhere1234567890abcdef1234567890ab")
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("expired API token", func(t *testing.T) {
		expiredRawToken := "dsh_expired1234567890abcdef1234567890abcdef123456"
		expiredHash := sha256.Sum256([]byte(expiredRawToken))
		expiredTokenHash := hex.EncodeToString(expiredHash[:])
		pastTime := time.Now().Add(-1 * time.Hour)

		expiredAPIToken := models.APIToken{
			UserID:    user.ID,
			Name:      "expired-token",
			TokenHash: expiredTokenHash,
			Prefix:    expiredRawToken[:8],
			ExpiresAt: &pastTime,
		}
		db.Create(&expiredAPIToken)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+expiredRawToken)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
		if body["error"] != "API token has expired" {
			t.Fatalf("expected expired error, got %v", body["error"])
		}
	})
}

func TestOptionalAuth(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	auth := NewAuthMiddleware(db)
	user, token := createMiddlewareTestUser(t, db, "optional-auth@test.com", models.UserRoleUser)

	app := fiber.New()
	app.Get("/public", auth.OptionalAuth, func(c *fiber.Ctx) error {
		u := GetCurrentUser(c)
		if u != nil {
			return c.JSON(fiber.Map{"authenticated": true, "email": u.Email})
		}
		return c.JSON(fiber.Map{"authenticated": false})
	})

	t.Run("no auth header proceeds without user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["authenticated"] != false {
			t.Fatalf("expected unauthenticated")
		}
	})

	t.Run("valid JWT sets user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["authenticated"] != true {
			t.Fatalf("expected authenticated")
		}
	})

	t.Run("invalid JWT proceeds without user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["authenticated"] != false {
			t.Fatalf("expected unauthenticated with bad token")
		}
	})

	t.Run("invalid format proceeds without user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.Header.Set("Authorization", "Basic creds")
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["authenticated"] != false {
			t.Fatalf("expected unauthenticated with Basic auth")
		}
	})

	t.Run("valid API token in optional auth", func(t *testing.T) {
		rawToken := "dsh_optional1234567890abcdef1234567890abcdef1234"
		hash := sha256.Sum256([]byte(rawToken))
		tokenHash := hex.EncodeToString(hash[:])
		apiToken := models.APIToken{
			UserID:    user.ID,
			Name:      "optional-test",
			TokenHash: tokenHash,
			Prefix:    rawToken[:8],
		}
		db.Create(&apiToken)

		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.Header.Set("Authorization", "Bearer "+rawToken)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if body["authenticated"] != true {
			t.Fatalf("expected authenticated with API token")
		}
	})
}

func TestAdminOnly(t *testing.T) {
	db := setupMiddlewareTestDB(t)
	auth := NewAuthMiddleware(db)
	_, adminToken := createMiddlewareTestUser(t, db, "admin@test.com", models.UserRoleAdmin)
	_, userToken := createMiddlewareTestUser(t, db, "user@test.com", models.UserRoleUser)

	app := fiber.New()
	app.Get("/admin", auth.RequireAuth, AdminOnly, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"admin": true})
	})

	t.Run("admin user can access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("regular user is forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
		if body["error"] != "admin access required" {
			t.Fatalf("expected admin required error, got %v", body["error"])
		}
	})

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestGetCurrentUser(t *testing.T) {
	t.Run("returns nil when no user set", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			user := GetCurrentUser(c)
			if user != nil {
				return c.JSON(fiber.Map{"error": "expected nil"})
			}
			return c.JSON(fiber.Map{"ok": true})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if body["ok"] != true {
			t.Fatalf("expected ok=true")
		}
	})

	t.Run("returns nil when locals contains wrong type", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			c.Locals(currentUserKey, "not-a-user")
			user := GetCurrentUser(c)
			if user != nil {
				return c.JSON(fiber.Map{"error": "expected nil"})
			}
			return c.JSON(fiber.Map{"ok": true})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, _ := app.Test(req, 5000)
		body := decodeBody(t, resp)
		if body["ok"] != true {
			t.Fatalf("expected ok=true")
		}
	})

	_ = uuid.Nil
}

func TestCORS(t *testing.T) {
	handler := CORS("http://localhost:3001")
	if handler == nil {
		t.Fatal("expected non-nil CORS handler")
	}
}
