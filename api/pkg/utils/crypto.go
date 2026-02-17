package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

var encryptionKey []byte

const encryptionSalt = "docshare-totp-encryption"

func ConfigureEncryption(secret string) {
	if secret == "" {
		return
	}
	hkdfReader := hkdf.New(
		sha256.New,
		[]byte(secret),
		[]byte(encryptionSalt),
		[]byte("encryption-key"),
	)
	encryptionKey = make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, encryptionKey); err != nil {
		panic(fmt.Sprintf("failed to derive encryption key: %v", err))
	}
}

func EncryptAESGCM(plaintext string) (string, error) {
	if encryptionKey == nil {
		return "", errors.New("encryption not configured")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptAESGCM(encrypted string) (string, error) {
	if encryptionKey == nil {
		return "", errors.New("encryption not configured")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func DecryptOrPlaintext(value string) string {
	if value == "" {
		return ""
	}
	decrypted, err := DecryptAESGCM(value)
	if err != nil {
		return value
	}
	return decrypted
}
