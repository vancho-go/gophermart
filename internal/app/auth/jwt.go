package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"net/http"
	"time"
)

const (
	tokenExp = time.Hour * 24
)

var secretKey string

type claims struct {
	jwt.RegisteredClaims
	UserID string
}

func newClaims() *claims {
	return &claims{}
}

func SetSecretKey(key string) error {
	secretKey = key
	return nil
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

func GetUserID(req *http.Request) (string, error) {

	cookie, err := req.Cookie("AuthToken")
	if err != nil {
		return "", fmt.Errorf("getUserID: cookie not found : %w", err)
	}

	tokenString := cookie.Value

	if err = isTokenValid(tokenString); err != nil {
		return "", fmt.Errorf("getUserID: error validating token : %w", err)
	}

	claims := newClaims()
	_, err = jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return "", fmt.Errorf("getUserID: error parsing token: %w", err)
	}
	return claims.UserID, nil
}

func isTokenValid(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("isTokenValid: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("isTokenValid: token is not valid")
	}
	return nil
}
