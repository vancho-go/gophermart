package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type key int

const (
	CookieKey key = iota
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("AuthToken")
		if err == nil && isTokenValid(cookie.Value) {
			next.ServeHTTP(res, req)
			return
		}

		userID := GenerateUserID()
		jwtToken, err := generateJWTToken(userID)
		if err != nil {
			slog.Error("error building new token", err)
			return
		}
		slog.Error("generated new jwt token for user %s", userID)
		cookieNew := &http.Cookie{
			Name:     "AuthToken",
			Value:    jwtToken,
			Expires:  time.Now().Add(tokenExp),
			HttpOnly: true,
			Path:     "/",
		}

		http.SetCookie(res, cookieNew)

		ctx := context.WithValue(req.Context(), CookieKey, cookieNew)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
