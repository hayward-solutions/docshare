package utils

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	t.Run("hashes password and validates original password", func(t *testing.T) {
		password := "super-secret-password"

		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("expected hashing to succeed, got error: %v", err)
		}
		if hash == "" {
			t.Fatal("expected non-empty hash, got empty string")
		}
		if hash == password {
			t.Fatal("expected hash to differ from raw password")
		}

		if !CheckPassword(password, hash) {
			t.Fatal("expected password check to succeed for matching password and hash")
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		hash, err := HashPassword("correct-password")
		if err != nil {
			t.Fatalf("failed to hash password for test: %v", err)
		}

		if CheckPassword("wrong-password", hash) {
			t.Fatal("expected password check to fail for wrong password")
		}
	})

	t.Run("supports empty password consistently", func(t *testing.T) {
		hash, err := HashPassword("")
		if err != nil {
			t.Fatalf("expected empty password hashing to succeed, got error: %v", err)
		}
		if hash == "" {
			t.Fatal("expected non-empty hash for empty password input")
		}

		if !CheckPassword("", hash) {
			t.Fatal("expected empty password to match its generated hash")
		}
		if CheckPassword("not-empty", hash) {
			t.Fatal("expected non-empty password to fail against empty-password hash")
		}
	})

	t.Run("returns false for malformed hash", func(t *testing.T) {
		if CheckPassword("anything", "not-a-valid-bcrypt-hash") {
			t.Fatal("expected malformed hash comparison to return false")
		}
	})
}
