package previewtoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/docshare/backend/pkg/logger"
)

const defaultTokenExpiry = 15 * time.Minute

var (
	secret []byte
	store  = &tokenStore{
		tokens: make(map[string]time.Time),
	}
)

type tokenStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time
}

type PreviewToken struct {
	FileID    string `json:"fid"`
	UserID    string `json:"uid"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nce"`
}

func SetSecret(s string) {
	secret = []byte(s)
}

func StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()
}

func Generate(fileID, userID string) string {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}

	tok := PreviewToken{
		FileID:    fileID,
		UserID:    userID,
		ExpiresAt: time.Now().Add(defaultTokenExpiry).Unix(),
		Nonce:     hex.EncodeToString(nonce),
	}

	data, err := json.Marshal(tok)
	if err != nil {
		return ""
	}

	encoded := base64.RawURLEncoding.EncodeToString(data)
	return encoded + "." + sign(data)
}

func Validate(tokenString string) (*PreviewToken, error) {
	dataPart, sigPart, err := split(tokenString)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.RawURLEncoding.DecodeString(dataPart)
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding")
	}

	if sign(decoded) != sigPart {
		return nil, fmt.Errorf("invalid token signature")
	}

	var tok PreviewToken
	if err := json.Unmarshal(decoded, &tok); err != nil {
		return nil, fmt.Errorf("invalid token data")
	}

	if time.Now().Unix() > tok.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	return &tok, nil
}

func GetMetadata(tokenString string) (fileID, userID string, err error) {
	tok, err := Validate(tokenString)
	if err != nil {
		return "", "", err
	}
	return tok.FileID, tok.UserID, nil
}

func MarkUsed(tokenString string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.tokens[tokenString] = time.Now()
}

func IsUsed(tokenString string) bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	_, exists := store.tokens[tokenString]
	logger.Info("token_is_used_check", map[string]interface{}{
		"token_last_20": tokenString[max(0, len(tokenString)-20):],
		"exists":        exists,
		"tokens_count":  len(store.tokens),
	})
	return exists
}

func cleanup() {
	store.mu.Lock()
	defer store.mu.Unlock()
	threshold := time.Now().Add(-defaultTokenExpiry)
	for key, usedAt := range store.tokens {
		if usedAt.Before(threshold) {
			delete(store.tokens, key)
		}
	}
}

func sign(data []byte) string {
	key := secret
	if len(key) == 0 {
		key = []byte("docshare-preview-token-fallback")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func split(tokenString string) (string, string, error) {
	for i := len(tokenString) - 1; i >= 0; i-- {
		if tokenString[i] == '.' {
			if i == len(tokenString)-1 {
				break
			}
			return tokenString[:i], tokenString[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid token format")
}
