package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const mfaTokenExpiry = 5 * time.Minute

type MFAClaims struct {
	UserID    uuid.UUID `json:"userID"`
	Email     string    `json:"email"`
	TokenType string    `json:"tokenType"`
	JTI       string    `json:"jti"`
	jwt.RegisteredClaims
}

func GenerateMFAToken(userID uuid.UUID, email string) (string, error) {
	expiresAt := time.Now().Add(mfaTokenExpiry)
	jti := uuid.New().String()
	claims := MFAClaims{
		UserID:    userID,
		Email:     email,
		TokenType: "mfa_challenge",
		JTI:       jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateMFAToken(tokenString string) (*MFAClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MFAClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*MFAClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid MFA token")
	}

	if claims.TokenType != "mfa_challenge" {
		return nil, fmt.Errorf("invalid token type")
	}

	if claims.JTI == "" {
		return nil, fmt.Errorf("missing token ID")
	}

	return claims, nil
}

var consumedJTIs = make(map[string]time.Time)
var jtiMu sync.Mutex

func IsJTIValid(jti string) bool {
	jtiMu.Lock()
	defer jtiMu.Unlock()
	_, exists := consumedJTIs[jti]
	return !exists
}

func ConsumeJTI(jti string) {
	jtiMu.Lock()
	defer jtiMu.Unlock()
	consumedJTIs[jti] = time.Now()
}

func CleanupExpiredJTIs() {
	jtiMu.Lock()
	defer jtiMu.Unlock()
	now := time.Now()
	for jti, consumedAt := range consumedJTIs {
		if now.Sub(consumedAt) > mfaTokenExpiry {
			delete(consumedJTIs, jti)
		}
	}
}
