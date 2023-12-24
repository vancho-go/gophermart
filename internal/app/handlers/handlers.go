package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/models"
	"github.com/vancho-go/gophermart/internal/app/storage"
	"io"
	"net/http"
)

type UserAuthenticator interface {
	RegisterUser(ctx context.Context, username, password string) (userID string, err error)
	AuthenticateUser(ctx context.Context, username, password string) (userID string, err error)
}

type OrderProcessor interface {
	AddOrder(ctx context.Context, order models.APIAddOrderRequest) (err error)
	GetOrders(ctx context.Context, userID string) (orders []models.APIGetOrderResponse, err error)
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

func AddOrder(op OrderProcessor) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		userID, err := auth.GetUserID(req)
		if err != nil {
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		orderNumber := string(body)

		ok, err := isOrderNumberValid(orderNumber)
		if !ok || err != nil {
			http.Error(res, "Incorrect order number format", http.StatusUnprocessableEntity)
			return
		}

		orderRequest := models.APIAddOrderRequest{OrderNumber: orderNumber, UserID: userID}

		err = op.AddOrder(req.Context(), orderRequest)
		if err != nil {
			if errors.Is(err, storage.ErrOrderNumberWasAlreadyAddedByThisUser) {
				http.Error(res, "Order number was already added", http.StatusOK)
				return
			} else if errors.Is(err, storage.ErrOrderNumberWasAlreadyAddedByAnotherUser) {
				http.Error(res, "Order number was already added", http.StatusConflict)
				return
			}
		}
		res.WriteHeader(http.StatusAccepted)
	}
}
