package utils

import (
	"testing"
	"time"

	"github.com/docshare/api/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateMFAToken(t *testing.T) {
	ConfigureJWT("test-secret", 24)

	userID := uuid.New()
	token, err := GenerateMFAToken(userID, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate MFA token: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ValidateMFAToken(token)
	if err != nil {
		t.Fatalf("failed to validate MFA token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected userID %s, got %s", userID, claims.UserID)
	}

	if claims.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", claims.Email)
	}

	if claims.TokenType != "mfa_challenge" {
		t.Fatalf("expected token type mfa_challenge, got %s", claims.TokenType)
	}

	if claims.JTI == "" {
		t.Fatal("expected JTI to be set")
	}
}

func TestValidateMFAToken_RejectsRegularJWT(t *testing.T) {
	ConfigureJWT("test-secret", 24)

	userID := uuid.New()
	regularClaims := Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, regularClaims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to generate regular JWT: %v", err)
	}

	_, err = ValidateMFAToken(tokenString)
	if err == nil {
		t.Fatal("expected error for regular JWT")
	}
}

func TestValidateMFAToken_Expired(t *testing.T) {
	ConfigureJWT("test-secret", 24)

	userID := uuid.New()
	expiredClaims := MFAClaims{
		UserID:    userID,
		Email:     "test@example.com",
		TokenType: "mfa_challenge",
		JTI:       uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = ValidateMFAToken(tokenString)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
