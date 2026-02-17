package previewtoken

import (
	"testing"
	"time"
)

func TestPreviewToken(t *testing.T) {
	SetSecret("test-secret-key")

	t.Run("Generate creates valid token", func(t *testing.T) {
		fileID := "file-123"
		userID := "user-456"

		token := Generate(fileID, userID)
		if token == "" {
			t.Fatal("expected non-empty token")
		}
	})

	t.Run("Validate returns token for valid string", func(t *testing.T) {
		fileID := "file-abc"
		userID := "user-xyz"

		token := Generate(fileID, userID)
		tok, err := Validate(token)
		if err != nil {
			t.Fatalf("expected valid token, got error: %v", err)
		}

		if tok.FileID != fileID {
			t.Errorf("expected FileID %s, got %s", fileID, tok.FileID)
		}
		if tok.UserID != userID {
			t.Errorf("expected UserID %s, got %s", userID, tok.UserID)
		}
		if tok.ExpiresAt <= time.Now().Unix() {
			t.Error("expected ExpiresAt to be in the future")
		}
	})

	t.Run("Validate rejects invalid format", func(t *testing.T) {
		_, err := Validate("invalid-token")
		if err == nil {
			t.Fatal("expected error for invalid token format")
		}
	})

	t.Run("Validate rejects token without dot", func(t *testing.T) {
		_, err := Validate("nodotinthisstring")
		if err == nil {
			t.Fatal("expected error for token without dot")
		}
	})

	t.Run("Validate rejects token with invalid signature", func(t *testing.T) {
		fileID := "file-sig-test"
		userID := "user-sig-test"

		token := Generate(fileID, userID)
		invalidToken := token + "tampered"

		_, err := Validate(invalidToken)
		if err == nil {
			t.Fatal("expected error for tampered token")
		}
	})

	t.Run("GetMetadata returns correct values", func(t *testing.T) {
		fileID := "file-metadata"
		userID := "user-metadata"

		token := Generate(fileID, userID)
		gotFileID, gotUserID, err := GetMetadata(token)
		if err != nil {
			t.Fatalf("expected valid metadata, got error: %v", err)
		}

		if gotFileID != fileID {
			t.Errorf("expected FileID %s, got %s", fileID, gotFileID)
		}
		if gotUserID != userID {
			t.Errorf("expected UserID %s, got %s", userID, gotUserID)
		}
	})

	t.Run("GetMetadata returns error for invalid token", func(t *testing.T) {
		_, _, err := GetMetadata("invalid")
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})
}

func TestSign(t *testing.T) {
	SetSecret("sign-test-secret")

	t.Run("sign produces consistent output", func(t *testing.T) {
		data := []byte("test data to sign")
		sig1 := sign(data)
		sig2 := sign(data)

		if sig1 != sig2 {
			t.Error("expected same signature for same data")
		}
	})

	t.Run("sign produces different output for different data", func(t *testing.T) {
		sig1 := sign([]byte("data1"))
		sig2 := sign([]byte("data2"))

		if sig1 == sig2 {
			t.Error("expected different signatures for different data")
		}
	})
}

func TestSplit(t *testing.T) {
	t.Run("split works correctly", func(t *testing.T) {
		data, sig, err := split("abc.def")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data != "abc" {
			t.Errorf("expected data 'abc', got %s", data)
		}
		if sig != "def" {
			t.Errorf("expected sig 'def', got %s", sig)
		}
	})

	t.Run("split returns error for no dot", func(t *testing.T) {
		_, _, err := split("nodot")
		if err == nil {
			t.Fatal("expected error for no dot")
		}
	})

	t.Run("split returns error for dot at end", func(t *testing.T) {
		_, _, err := split("dotatend.")
		if err == nil {
			t.Fatal("expected error for dot at end")
		}
	})

	t.Run("split handles multiple dots", func(t *testing.T) {
		data, sig, err := split("a.b.c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data != "a.b" {
			t.Errorf("expected data 'a.b', got %s", data)
		}
		if sig != "c" {
			t.Errorf("expected sig 'c', got %s", sig)
		}
	})
}
