package utils

import (
	"testing"
)

func TestConfigureEncryption(t *testing.T) {
	tests := []struct {
		name       string
		secret     string
		wantKeySet bool
	}{
		{
			name:       "empty secret does not set key",
			secret:     "",
			wantKeySet: false,
		},
		{
			name:       "valid secret sets key",
			secret:     "test-secret-key-32-bytes-long!!",
			wantKeySet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encryptionKey = nil
			ConfigureEncryption(tt.secret)

			if tt.wantKeySet && encryptionKey == nil {
				t.Error("expected encryption key to be set")
			}
			if !tt.wantKeySet && encryptionKey != nil {
				t.Error("expected encryption key to not be set")
			}
		})
	}
}

func TestEncryptAESGCM(t *testing.T) {
	ConfigureEncryption("test-encryption-secret-32-bytes-long!!")

	tests := []struct {
		name      string
		plaintext string
		wantErr   bool
	}{
		{
			name:      "empty plaintext encrypts successfully",
			plaintext: "",
			wantErr:   false,
		},
		{
			name:      "normal plaintext encrypts successfully",
			plaintext: "hello world",
			wantErr:   false,
		},
		{
			name:      "unicode plaintext encrypts successfully",
			plaintext: "hello 世界",
			wantErr:   false,
		},
		{
			name:      "long plaintext encrypts successfully",
			plaintext: string(make([]byte, 1000)),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := EncryptAESGCM(tt.plaintext)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncryptAESGCM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ciphertext == "" {
				t.Error("expected non-empty ciphertext")
			}
		})
	}
}

func TestDecryptAESGCM(t *testing.T) {
	ConfigureEncryption("test-encryption-secret-32-bytes-long!!")

	originalText := "hello world"
	ciphertext, err := EncryptAESGCM(originalText)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	tests := []struct {
		name       string
		ciphertext string
		wantPlain  string
		wantErr    bool
		setupKey   string
	}{
		{
			name:       "valid ciphertext decrypts correctly",
			ciphertext: ciphertext,
			wantPlain:  originalText,
			wantErr:    false,
			setupKey:   "test-encryption-secret-32-bytes-long!!",
		},
		{
			name:       "invalid base64 returns error",
			ciphertext: "not-valid-base64!!!",
			wantPlain:  "",
			wantErr:    true,
			setupKey:   "test-encryption-secret-32-bytes-long!!",
		},
		{
			name:       "invalid ciphertext too short",
			ciphertext: "YWJj", // "abc" in base64
			wantPlain:  "",
			wantErr:    true,
			setupKey:   "test-encryption-secret-32-bytes-long!!",
		},
		{
			name:       "wrong key produces error",
			ciphertext: ciphertext,
			wantPlain:  "",
			wantErr:    true,
			setupKey:   "different-key-32-bytes-long!!!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConfigureEncryption(tt.setupKey)
			plaintext, err := DecryptAESGCM(tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecryptAESGCM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && plaintext != tt.wantPlain {
				t.Errorf("DecryptAESGCM() = %v, want %v", plaintext, tt.wantPlain)
			}
		})
	}
}

func TestDecryptOrPlaintext(t *testing.T) {
	ConfigureEncryption("test-encryption-secret-32-bytes-long!!")

	encrypted, err := EncryptAESGCM("secret")
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	tests := []struct {
		name       string
		value      string
		wantReturn string
	}{
		{
			name:       "empty string returns empty",
			value:      "",
			wantReturn: "",
		},
		{
			name:       "encrypted value decrypts",
			value:      encrypted,
			wantReturn: "secret",
		},
		{
			name:       "plaintext value returns as-is",
			value:      "plaintext",
			wantReturn: "plaintext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecryptOrPlaintext(tt.value)
			if result != tt.wantReturn {
				t.Errorf("DecryptOrPlaintext() = %v, want %v", result, tt.wantReturn)
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	ConfigureEncryption("test-encryption-secret-32-bytes-long!!")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "short text",
			content: "hello",
		},
		{
			name:    "long text",
			content: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
		},
		{
			name:    "unicode",
			content: "你好世界🌍",
		},
		{
			name:    "special characters",
			content: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name:    "binary-like",
			content: string([]byte{0, 1, 2, 255, 128, 64, 32, 16, 8, 4, 2, 1}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptAESGCM(tt.content)
			if err != nil {
				t.Fatalf("EncryptAESGCM() error = %v", err)
			}

			decrypted, err := DecryptAESGCM(encrypted)
			if err != nil {
				t.Fatalf("DecryptAESGCM() error = %v", err)
			}

			if decrypted != tt.content {
				t.Errorf("round trip failed: got %v, want %v", decrypted, tt.content)
			}
		})
	}
}
