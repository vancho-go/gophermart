package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"net/http"
	"time"
)

const tokenExp = time.Hour * 24
const secretKey = "temp_secret_key"

type claims struct {
	jwt.RegisteredClaims
	UserID string
}

func GenerateUserID() string {
	return uuid.New().String()
}

func GenerateCookie(userID string) (*http.Cookie, error) {
	jwtToken, err := generateJWTToken(userID)
	if err != nil {
		return nil, fmt.Errorf("generateCookie: error generating cookie: %w", err)
	}
	return &http.Cookie{
		Name:     "AuthToken",
		Value:    jwtToken,
		Expires:  time.Now().Add(tokenExp),
		HttpOnly: true,
		Path:     "/",
	}, nil
}

func generateJWTToken(userID string) (string, error) {
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	expirationTime := time.Now().Add(tokenExp)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
			},
			UserID: userID,
		})
	return token.SignedString([]byte(secretKey))
}
