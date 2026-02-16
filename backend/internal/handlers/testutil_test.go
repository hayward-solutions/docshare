package handlers

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	gosqlite "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"gorm.io/gorm"
)

type testEnv struct {
	app *fiber.App
	db  *gorm.DB
}

var testSetupOnce sync.Once

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	testSetupOnce.Do(func() {
		gosqlite.MustRegisterScalarFunction("NOW", 0, func(ctx *gosqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return time.Now().UTC(), nil
		})
		logger.Init()
		utils.ConfigureJWT("test-secret", 24)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed getting sql.DB from gorm: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMembership{},
		&models.File{},
		&models.Share{},
		&models.Activity{},
		&models.APIToken{},
		&models.DeviceCode{},
		&models.AuditLog{},
		&models.AuditExportCursor{},
		&models.Transfer{},
		&models.SSOProvider{},
		&models.LinkedAccount{},
		&models.PreviewJob{},
	)
	if err != nil {
		t.Fatalf("failed automigrating models: %v", err)
	}

	accessService := services.NewAccessService(db)
	previewService := services.NewPreviewService(db, nil, config.GotenbergConfig{})
	previewQueueService := services.NewPreviewQueueService(db, previewService, config.PreviewConfig{
		QueueBufferSize: 10,
		MaxAttempts:     3,
		RetryDelays:     []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute},
	})
	auditService := services.NewAuditService(db, nil)

	cfg := &config.Config{
		Server: config.ServerConfig{
			FrontendURL: "http://localhost:3001",
		},
		SSO: config.SSOConfig{
			AutoRegister: true,
			DefaultRole:  "user",
		},
	}

	authHandler := NewAuthHandler(db, auditService)
	usersHandler := NewUsersHandler(db, auditService)
	groupsHandler := NewGroupsHandler(db, auditService)
	filesHandler := NewFilesHandler(db, nil, accessService, previewService, previewQueueService, auditService)
	sharesHandler := NewSharesHandler(db, accessService, auditService)
	activitiesHandler := NewActivitiesHandler(db)
	auditHandler := NewAuditHandler(db)
	apiTokenHandler := NewAPITokenHandler(db, auditService)
	deviceAuthHandler := NewDeviceAuthHandler(db, auditService, cfg)
	transfersHandler := NewTransfersHandler(db, 300)
	authMiddleware := middleware.NewAuthMiddleware(db)

	ssoHandler := NewSSOHandler(db, cfg)

	app := fiber.New(fiber.Config{BodyLimit: 100 * 1024 * 1024})
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(middleware.CORS())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.SecurityLogger())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")
	api.Get("/version", GetVersion)

	authRoutes := api.Group("/auth")
	authRoutes.Post("/register", authHandler.Register)
	authRoutes.Post("/login", authHandler.Login)
	authRoutes.Get("/me", authMiddleware.RequireAuth, authHandler.Me)
	authRoutes.Put("/me", authMiddleware.RequireAuth, authHandler.UpdateMe)
	authRoutes.Put("/password", authMiddleware.RequireAuth, authHandler.ChangePassword)

	api.Get("/users/search", authMiddleware.RequireAuth, usersHandler.Search)

	userRoutes := api.Group("/users", authMiddleware.RequireAuth, middleware.AdminOnly)
	userRoutes.Get("/", usersHandler.List)
	userRoutes.Get("/:id", usersHandler.Get)
	userRoutes.Put("/:id", usersHandler.Update)
	userRoutes.Delete("/:id", usersHandler.Delete)

	groupRoutes := api.Group("/groups", authMiddleware.RequireAuth)
	groupRoutes.Post("/", groupsHandler.Create)
	groupRoutes.Get("/", groupsHandler.List)
	groupRoutes.Get("/:id", groupsHandler.Get)
	groupRoutes.Put("/:id", groupsHandler.Update)
	groupRoutes.Delete("/:id", groupsHandler.Delete)
	groupRoutes.Post("/:id/members", groupsHandler.AddMember)
	groupRoutes.Delete("/:id/members/:userId", groupsHandler.RemoveMember)
	groupRoutes.Put("/:id/members/:userId", groupsHandler.UpdateMemberRole)

	api.Get("/files/:id/proxy", filesHandler.ProxyPreview)

	publicFileRoutes := api.Group("/public/files", authMiddleware.OptionalAuth)
	publicFileRoutes.Get("/:id", filesHandler.PublicGet)
	publicFileRoutes.Get("/:id/download", filesHandler.PublicDownload)
	publicFileRoutes.Get("/:id/children", filesHandler.PublicChildren)

	fileRoutes := api.Group("/files", authMiddleware.RequireAuth)
	fileRoutes.Post("/upload", filesHandler.Upload)
	fileRoutes.Post("/directory", filesHandler.CreateDirectory)
	fileRoutes.Get("/", filesHandler.ListRoot)
	fileRoutes.Get("/search", filesHandler.Search)
	fileRoutes.Get("/:id/children", filesHandler.ListChildren)
	fileRoutes.Get("/:id/download", filesHandler.Download)
	fileRoutes.Get("/:id/download-url", filesHandler.DownloadURL)
	fileRoutes.Get("/:id/preview", filesHandler.PreviewURL)
	fileRoutes.Get("/:id/convert-preview", filesHandler.ConvertPreview)
	fileRoutes.Get("/:id/preview-status", filesHandler.PreviewStatus)
	fileRoutes.Get("/:id/retry-preview", filesHandler.RetryPreview)
	fileRoutes.Get("/:id/path", filesHandler.Path)
	fileRoutes.Post("/:id/share", sharesHandler.ShareFile)
	fileRoutes.Get("/:id/shares", sharesHandler.ListFileShares)
	fileRoutes.Get("/:id", filesHandler.Get)
	fileRoutes.Put("/:id", filesHandler.Update)
	fileRoutes.Delete("/:id", filesHandler.Delete)

	shareRoutes := api.Group("/shares", authMiddleware.RequireAuth)
	shareRoutes.Delete("/:id", sharesHandler.DeleteShare)
	shareRoutes.Put("/:id", sharesHandler.UpdateShare)

	api.Get("/shared", authMiddleware.RequireAuth, sharesHandler.ListSharedWithMe)

	activityRoutes := api.Group("/activities", authMiddleware.RequireAuth)
	activityRoutes.Get("/", activitiesHandler.List)
	activityRoutes.Get("/unread-count", activitiesHandler.UnreadCount)
	activityRoutes.Put("/read-all", activitiesHandler.MarkAllRead)
	activityRoutes.Put("/:id/read", activitiesHandler.MarkRead)

	tokenRoutes := api.Group("/auth/tokens", authMiddleware.RequireAuth)
	tokenRoutes.Post("/", apiTokenHandler.Create)
	tokenRoutes.Get("/", apiTokenHandler.List)
	tokenRoutes.Delete("/:id", apiTokenHandler.Revoke)

	deviceRoutes := api.Group("/auth/device")
	deviceRoutes.Post("/code", deviceAuthHandler.RequestCode)
	deviceRoutes.Post("/token", deviceAuthHandler.PollToken)
	deviceRoutes.Get("/verify", authMiddleware.RequireAuth, deviceAuthHandler.Verify)
	deviceRoutes.Post("/approve", authMiddleware.RequireAuth, deviceAuthHandler.Approve)

	auditRoutes := api.Group("/audit-log", authMiddleware.RequireAuth)
	auditRoutes.Get("/export", auditHandler.ExportMyLog)

	transferRoutes := api.Group("/transfers", authMiddleware.RequireAuth)
	transferRoutes.Post("/", transfersHandler.Create)
	transferRoutes.Get("/", transfersHandler.List)
	transferRoutes.Get("/:code", transfersHandler.Get)
	transferRoutes.Post("/:code/connect", transfersHandler.Connect)
	transferRoutes.Post("/:code/upload", transfersHandler.Upload)
	transferRoutes.Get("/:code/download", transfersHandler.Download)
	transferRoutes.Post("/:code/complete", transfersHandler.Complete)
	transferRoutes.Delete("/:code", transfersHandler.Cancel)

	ssoRoutes := api.Group("/auth/sso")
	ssoRoutes.Get("/providers", ssoHandler.ListProviders)
	ssoRoutes.Get("/oauth/:provider", ssoHandler.GetLoginRedirect)
	ssoRoutes.Get("/oauth/:provider/callback", ssoHandler.HandleOAuthCallback)
	ssoRoutes.Post("/ldap/login", ssoHandler.HandleLDAPLogin)

	ssoProtectedRoutes := api.Group("/auth/sso", authMiddleware.RequireAuth)
	ssoProtectedRoutes.Get("/linked-accounts", ssoHandler.GetLinkedAccounts)
	ssoProtectedRoutes.Delete("/linked-accounts/:id", ssoHandler.UnlinkAccount)

	return &testEnv{app: app, db: db}
}

func createTestUser(t *testing.T, db *gorm.DB, email, password string, role models.UserRole) (*models.User, string) {
	t.Helper()

	hash, err := utils.HashPassword(password)
	if err != nil {
		t.Fatalf("failed hashing password: %v", err)
	}

	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		FirstName:    "Test",
		LastName:     "User",
		Role:         role,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed creating test user: %v", err)
	}

	token, err := utils.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed generating auth token: %v", err)
	}

	return user, token
}

func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func performRequest(t *testing.T, app *fiber.App, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := app.Test(req, int((10 * time.Second).Milliseconds()))
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
			t.Fatalf("failed to marshal payload: %v", err)
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
		t.Fatalf("failed decoding JSON response: %v body=%q", err, string(raw))
	}

	return payload
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Fatalf("expected status %d, got %d", expected, resp.StatusCode)
	}
}

func assertEnvelopeError(t *testing.T, body map[string]any, expected string) {
	t.Helper()
	if success, _ := body["success"].(bool); success {
		t.Fatalf("expected success=false, got %+v", body)
	}
	if got, _ := body["error"].(string); got != expected {
		t.Fatalf("expected error %q, got %q", expected, got)
	}
}
