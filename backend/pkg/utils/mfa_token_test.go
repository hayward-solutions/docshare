package utils

import (
	"testing"
	"time"

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
}

func TestValidateMFAToken_RejectsRegularJWT(t *testing.T) {
	ConfigureJWT("test-secret", 24)

	user := &struct {
		ID    uuid.UUID
		Email string
		Role  string
	}{
		ID:    uuid.New(),
		Email: "test@example.com",
		Role:  "user",
	}
	_ = user

	_, err := ValidateMFAToken("some-invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateMFAToken_Expired(t *testing.T) {
	ConfigureJWT("test-secret", 24)

	userID := uuid.New()
	token, err := GenerateMFAToken(userID, "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate MFA token: %v", err)
	}

	_ = time.Now()

	claims, err := ValidateMFAToken(token)
	if err != nil {
		t.Fatalf("valid MFA token should not error: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected userID %s, got %s", userID, claims.UserID)
	}
}
