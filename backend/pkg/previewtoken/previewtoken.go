package previewtoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const defaultTokenExpiry = 15 * time.Minute

var secret []byte

type PreviewToken struct {
	FileID    string `json:"fid"`
	UserID    string `json:"uid"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nce"`
}

func SetSecret(s string) {
	secret = []byte(s)
}

func StartCleanup(_ time.Duration) {
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
