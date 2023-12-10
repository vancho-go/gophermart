package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/vancho-go/gophermart/internal/app/models"
	"github.com/vancho-go/gophermart/internal/app/storage"
	"net/http"
)

type UserAuthenticator interface {
	RegisterUser(context.Context, string, string) (*http.Cookie, error)
	AuthenticateUser(context.Context, string, string) error
}

func UserRegister(ua UserAuthenticator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var request models.APIRegisterRequest

		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&request); err != nil {
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		cookie, err := ua.RegisterUser(req.Context(), request.Login, request.Password)
		if errors.Is(err, storage.ErrUsernameNotUnique) {
			http.Error(res, "Username is already in use", http.StatusConflict)
			return
		} else if err != nil {
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(res, cookie)
		res.WriteHeader(http.StatusOK)
	}
}
