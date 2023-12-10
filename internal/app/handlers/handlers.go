package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/models"
	"github.com/vancho-go/gophermart/internal/app/storage"
	"net/http"
)

type UserAuthenticator interface {
	RegisterUser(context.Context, string, string) (string, error)
	AuthenticateUser(context.Context, string, string) (string, error)
}

func RegisterUser(ua UserAuthenticator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var request models.APIRegisterRequest

		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&request); err != nil {
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		userID, err := ua.RegisterUser(req.Context(), request.Login, request.Password)
		if errors.Is(err, storage.ErrUsernameNotUnique) {
			http.Error(res, "Username is already in use", http.StatusConflict)
			return
		} else if err != nil {
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		cookie, err := auth.GenerateCookie(userID)
		if err != nil {
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(res, cookie)
		res.WriteHeader(http.StatusOK)
	}
}

func AuthenticateUser(ua UserAuthenticator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var request models.APIAuthRequest

		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&request); err != nil {
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		userID, err := ua.AuthenticateUser(req.Context(), request.Login, request.Password)
		if errors.Is(err, storage.ErrUserNotFound) {
			http.Error(res, "Wrong username or password", http.StatusUnauthorized)
			return
		} else if err != nil {
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		cookie, err := auth.GenerateCookie(userID)
		if err != nil {
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(res, cookie)
		res.WriteHeader(http.StatusOK)
	}
}
