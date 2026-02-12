package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/database"
	"github.com/docshare/backend/internal/handlers"
	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/services"
	"github.com/docshare/backend/internal/storage"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/previewtoken"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	logger.Init()

	cfg := config.Load()
	utils.ConfigureJWT(cfg.JWT.Secret, cfg.JWT.ExpirationHours)
	previewtoken.SetSecret(cfg.JWT.Secret)
	previewtoken.StartCleanup(5 * time.Minute)

	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	storageClient, err := storage.NewMinIOClient(cfg.MinIO)
	if err != nil {
		log.Fatalf("minio initialization failed: %v", err)
	}
	if err := storageClient.EnsureBucket(context.Background()); err != nil {
		log.Fatalf("failed ensuring minio bucket: %v", err)
	}

	accessService := services.NewAccessService(db)
	previewService := services.NewPreviewService(db, storageClient, cfg.Gotenberg)

	authHandler := handlers.NewAuthHandler(db)
	usersHandler := handlers.NewUsersHandler(db)
	groupsHandler := handlers.NewGroupsHandler(db)
	filesHandler := handlers.NewFilesHandler(db, storageClient, accessService, previewService)
	sharesHandler := handlers.NewSharesHandler(db, accessService)

	authMiddleware := middleware.NewAuthMiddleware(db)

	app := fiber.New(fiber.Config{BodyLimit: 100 * 1024 * 1024})
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(middleware.CORS())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.SecurityLogger())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")

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

	listenAddr := fmt.Sprintf(":%s", cfg.Server.Port)

	logger.Info("server_starting", map[string]interface{}{
		"port":       cfg.Server.Port,
		"address":    listenAddr,
		"body_limit": "100MB",
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen(listenAddr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("shutting down server due to signal: %s", sig)
		shutdownDone := make(chan struct{})
		go func() {
			_ = app.Shutdown()
			close(shutdownDone)
		}()
		select {
		case <-shutdownDone:
		case <-time.After(10 * time.Second):
			log.Print("forced shutdown timeout reached")
		}
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
