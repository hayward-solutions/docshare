package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const mfaTokenExpiry = 5 * time.Minute

type MFAClaims struct {
	UserID    uuid.UUID `json:"userID"`
	Email     string    `json:"email"`
	TokenType string    `json:"tokenType"`
	jwt.RegisteredClaims
}

func GenerateMFAToken(userID uuid.UUID, email string) (string, error) {
	expiresAt := time.Now().Add(mfaTokenExpiry)
	claims := MFAClaims{
		UserID:    userID,
		Email:     email,
		TokenType: "mfa_challenge",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
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

	return claims, nil
}
