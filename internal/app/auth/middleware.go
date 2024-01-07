package auth

import (
	"context"
	"net/http"
)

type contextKey int

const (
	UserIDContextKey contextKey = iota
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		userID, err := GetUserID(req)
		if err != nil {
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(req.Context(), UserIDContextKey, userID)
		req = req.WithContext(ctx)

		next.ServeHTTP(res, req)
	})
}
