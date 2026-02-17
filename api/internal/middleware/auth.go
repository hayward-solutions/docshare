package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"gorm.io/gorm"
)

const currentUserKey = "currentUser"
const apiTokenPrefix = "dsh_"

type AuthMiddleware struct {
	DB *gorm.DB
}

func NewAuthMiddleware(db *gorm.DB) *AuthMiddleware {
	return &AuthMiddleware{DB: db}
}

func CORS(frontendURL string) fiber.Handler {
	origins := frontendURL
	if strings.Contains(frontendURL, "localhost") {
		loopback := strings.Replace(frontendURL, "localhost", "127.0.0.1", 1)
		origins = frontendURL + "," + loopback
	}
	return cors.New(cors.Config{
		AllowOrigins: origins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	})
}

func (a *AuthMiddleware) RequireAuth(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		logger.Warn("auth_missing_header", map[string]interface{}{
			"ip":   c.IP(),
			"path": c.Path(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "missing authorization header")
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if tokenString == authHeader || tokenString == "" {
		logger.Warn("auth_invalid_format", map[string]interface{}{
			"ip":          c.IP(),
			"path":        c.Path(),
			"auth_header": authHeader[:min(len(authHeader), 20)] + "...",
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid authorization format")
	}

	if strings.HasPrefix(tokenString, apiTokenPrefix) {
		return a.authenticateAPIToken(c, tokenString)
	}

	return a.authenticateJWT(c, tokenString)
}

func (a *AuthMiddleware) authenticateJWT(c *fiber.Ctx, tokenString string) error {
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

func (a *AuthMiddleware) authenticateAPIToken(c *fiber.Ctx, rawToken string) error {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	var apiToken models.APIToken
	if err := a.DB.First(&apiToken, "token_hash = ?", tokenHash).Error; err != nil {
		logger.Warn("api_token_not_found", map[string]interface{}{
			"ip":   c.IP(),
			"path": c.Path(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "invalid API token")
	}

	if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
		logger.Warn("api_token_expired", map[string]interface{}{
			"ip":       c.IP(),
			"path":     c.Path(),
			"token_id": apiToken.ID.String(),
		})
		return utils.Error(c, fiber.StatusUnauthorized, "API token has expired")
	}

	var user models.User
	if err := a.DB.First(&user, "id = ?", apiToken.UserID).Error; err != nil {
		return utils.Error(c, fiber.StatusUnauthorized, "user not found")
	}

	now := time.Now()
	a.DB.Model(&apiToken).Update("last_used_at", now)

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

	if strings.HasPrefix(tokenString, apiTokenPrefix) {
		hash := sha256.Sum256([]byte(tokenString))
		tokenHash := hex.EncodeToString(hash[:])

		var apiToken models.APIToken
		if err := a.DB.First(&apiToken, "token_hash = ?", tokenHash).Error; err != nil {
			return c.Next()
		}
		if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
			return c.Next()
		}

		var user models.User
		if err := a.DB.First(&user, "id = ?", apiToken.UserID).Error; err != nil {
			return c.Next()
		}

		now := time.Now()
		a.DB.Model(&apiToken).Update("last_used_at", now)
		c.Locals(currentUserKey, &user)
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
