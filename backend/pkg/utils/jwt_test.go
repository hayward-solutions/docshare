package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/docshare/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func configureJWTForTest(t *testing.T, secret string, expirationHours int) {
	t.Helper()

	originalSecret := append([]byte(nil), jwtSecret...)
	originalExpiration := jwtExpirationHours

	t.Cleanup(func() {
		jwtSecret = originalSecret
		jwtExpirationHours = originalExpiration
	})

	ConfigureJWT(secret, expirationHours)
}

func TestConfigureJWT(t *testing.T) {
	t.Run("updates secret and expiration when valid values are provided", func(t *testing.T) {
		configureJWTForTest(t, "test-secret", 72)

		if got := string(jwtSecret); got != "test-secret" {
			t.Fatalf("expected jwt secret to be %q, got %q", "test-secret", got)
		}
		if jwtExpirationHours != 72 {
			t.Fatalf("expected jwt expiration to be %d, got %d", 72, jwtExpirationHours)
		}
	})

	t.Run("ignores empty secret and non-positive expiration", func(t *testing.T) {
		configureJWTForTest(t, "initial-secret", 24)

		ConfigureJWT("", 0)

		if got := string(jwtSecret); got != "initial-secret" {
			t.Fatalf("expected jwt secret to remain %q, got %q", "initial-secret", got)
		}
		if jwtExpirationHours != 24 {
			t.Fatalf("expected jwt expiration to remain %d, got %d", 24, jwtExpirationHours)
		}
	})
}

func TestGenerateAndValidateToken(t *testing.T) {
	t.Run("generates and validates token for a user", func(t *testing.T) {
		configureJWTForTest(t, "roundtrip-secret", 1)

		user := &models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "user@example.com",
			Role:      models.UserRoleUser,
		}

		token, err := GenerateToken(user)
		if err != nil {
			t.Fatalf("expected token generation to succeed, got error: %v", err)
		}

		claims, err := ValidateToken(token)
		if err != nil {
			t.Fatalf("expected token validation to succeed, got error: %v", err)
		}

		if claims.UserID != user.ID {
			t.Fatalf("expected claims userID %s, got %s", user.ID, claims.UserID)
		}
		if claims.Email != user.Email {
			t.Fatalf("expected claims email %q, got %q", user.Email, claims.Email)
		}
		if claims.Role != user.Role {
			t.Fatalf("expected claims role %q, got %q", user.Role, claims.Role)
		}
		if claims.Subject != user.ID.String() {
			t.Fatalf("expected subject %q, got %q", user.ID.String(), claims.Subject)
		}
		if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
			t.Fatalf("expected token to have a future expiration, got %v", claims.ExpiresAt)
		}
	})

	t.Run("rejects expired token", func(t *testing.T) {
		configureJWTForTest(t, "expired-secret", 1)

		expiredClaims := Claims{
			UserID: uuid.New(),
			Email:  "expired@example.com",
			Role:   models.UserRoleUser,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				Subject:   uuid.New().String(),
			},
		}

		expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(jwtSecret)
		if err != nil {
			t.Fatalf("failed to sign expired token for test: %v", err)
		}

		if _, err := ValidateToken(expiredToken); err == nil {
			t.Fatal("expected expired token validation to fail, but it succeeded")
		}
	})

	t.Run("rejects malformed token string", func(t *testing.T) {
		configureJWTForTest(t, "malformed-secret", 1)

		if _, err := ValidateToken("not-a-jwt"); err == nil {
			t.Fatal("expected malformed token validation to fail, but it succeeded")
		}
	})

	t.Run("rejects token signed with unexpected method", func(t *testing.T) {
		configureJWTForTest(t, "wrong-method-secret", 1)

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate rsa key for test: %v", err)
		}

		rsaToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Subject:   uuid.New().String(),
		})

		signedToken, err := rsaToken.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign rsa token for test: %v", err)
		}

		_, err = ValidateToken(signedToken)
		if err == nil {
			t.Fatal("expected validation to fail for token with unexpected signing method")
		}
		if !strings.Contains(err.Error(), "unexpected signing method") {
			t.Fatalf("expected signing method error, got: %v", err)
		}
	})
}
