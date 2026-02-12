package middleware

import (
	"strings"

	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"gorm.io/gorm"
)

const currentUserKey = "currentUser"

type AuthMiddleware struct {
	DB *gorm.DB
}

func NewAuthMiddleware(db *gorm.DB) *AuthMiddleware {
	return &AuthMiddleware{DB: db}
}

func CORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: "http://localhost:3001,http://127.0.0.1:3001",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	})
}

func (a *AuthMiddleware) RequireAuth(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		logger.Warn("jwt_missing_header", map[string]interface{}{
			"ip":   c.IP(),
			"path": c.Path(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "missing authorization header")
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if tokenString == authHeader || tokenString == "" {
		logger.Warn("jwt_invalid_format", map[string]interface{}{
			"ip":          c.IP(),
			"path":        c.Path(),
			"auth_header": authHeader[:min(len(authHeader), 20)] + "...",
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid authorization format")
	}

	claims, err := utils.ValidateToken(tokenString)
	if err != nil {
		logger.Warn("jwt_validation_failed", map[string]interface{}{
			"ip":    c.IP(),
			"path":  c.Path(),
			"error": err.Error(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid or expired token")
	}

	var user models.User
	if err := a.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		logger.Warn("jwt_user_not_found", map[string]interface{}{
			"ip":      c.IP(),
			"path":    c.Path(),
			"user_id": claims.UserID,
		})
		return utils.Error(c, fiber.StatusUnauthorized, "user not found")
	}

	c.Locals(currentUserKey, &user)
	return c.Next()
}

func (a *AuthMiddleware) OptionalAuth(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Next()
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if tokenString == authHeader || tokenString == "" {
		return c.Next()
	}

	claims, err := utils.ValidateToken(tokenString)
	if err != nil {
		return c.Next()
	}

	var user models.User
	if err := a.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return c.Next()
	}

	c.Locals(currentUserKey, &user)
	return c.Next()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func AdminOnly(c *fiber.Ctx) error {
	user := GetCurrentUser(c)
	if user == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	if user.Role != models.UserRoleAdmin {
		return utils.Error(c, fiber.StatusForbidden, "admin access required")
	}
	return c.Next()
}

func GetCurrentUser(c *fiber.Ctx) *models.User {
	value := c.Locals(currentUserKey)
	if value == nil {
		return nil
	}
	user, ok := value.(*models.User)
	if !ok {
		return nil
	}
	return user
}
