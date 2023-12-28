package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/logger"
	"github.com/vancho-go/gophermart/internal/app/models"
	"github.com/vancho-go/gophermart/internal/app/storage"
	"go.uber.org/zap"
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

type BonusesProcessor interface {
	GetCurrentBonusesAmount(ctx context.Context, userID string) (bonuses models.APIGetBonusesAmountResponse, err error)
	UseBonuses(ctx context.Context, request models.APIUseBonusesRequest, userID string) (err error)
}

type WithdrawalsProcessor interface {
	GetWithdrawalsHistory(ctx context.Context, userID string) (withdrawals []models.APIGetWithdrawalsHistoryResponse, err error)
}

func getUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(auth.UserIDContextKey).(string)
	return userID, ok
}

func RegisterUser(ua UserAuthenticator, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var request models.APIRegisterRequest

		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&request); err != nil {
			logger.Info("registerUser:", zap.Error(err))
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		userID, err := ua.RegisterUser(req.Context(), request.Login, request.Password)
		if errors.Is(err, storage.ErrUsernameNotUnique) {
			logger.Info("registerUser:", zap.Error(err))
			http.Error(res, "Username is already in use", http.StatusConflict)
			return
		} else if err != nil {
			logger.Error("registerUser:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		cookie, err := auth.GenerateCookie(userID)
		if err != nil {
			logger.Error("registerUser:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(res, cookie)
		res.WriteHeader(http.StatusOK)
	}
}

func AuthenticateUser(ua UserAuthenticator, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var request models.APIAuthRequest

		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&request); err != nil {
			logger.Info("authenticateUser:", zap.Error(err))
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		userID, err := ua.AuthenticateUser(req.Context(), request.Login, request.Password)
		if errors.Is(err, storage.ErrUserNotFound) {
			logger.Info("authenticateUser:", zap.Error(err))
			http.Error(res, "Wrong username or password", http.StatusUnauthorized)
			return
		} else if err != nil {
			logger.Info("authenticateUser:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		cookie, err := auth.GenerateCookie(userID)
		if err != nil {
			logger.Error("authenticateUser:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(res, cookie)
		res.WriteHeader(http.StatusOK)
	}
}

func AddOrder(op OrderProcessor, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		userID, ok := getUserIDFromContext(req.Context())
		if !ok {
			logger.Info("addOrder: unauthorized")
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			logger.Info("authenticateUser:", zap.Error(err))
			http.Error(res, "Invalid request format", http.StatusBadRequest)
			return
		}

		orderNumber := string(body)

		ok, err = isOrderNumberValid(orderNumber)
		if !ok || err != nil {
			logger.Info("authenticateUser:", zap.Error(err))
			http.Error(res, "Incorrect order number format", http.StatusUnprocessableEntity)
			return
		}

		orderRequest := models.APIAddOrderRequest{OrderNumber: orderNumber, UserID: userID}

		err = op.AddOrder(req.Context(), orderRequest)
		if err != nil {
			if errors.Is(err, storage.ErrOrderNumberWasAlreadyAddedByThisUser) {
				logger.Info("authenticateUser:", zap.Error(err))
				http.Error(res, "Order number was already added", http.StatusOK)
				return
			} else if errors.Is(err, storage.ErrOrderNumberWasAlreadyAddedByAnotherUser) {
				logger.Info("authenticateUser:", zap.Error(err))
				http.Error(res, "Order number was already added", http.StatusConflict)
				return
			}
		}
		res.WriteHeader(http.StatusAccepted)
	}
}

func GetOrdersList(op OrderProcessor, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		userID, ok := getUserIDFromContext(req.Context())
		if !ok {
			logger.Info("getOrdersList: unauthorized")
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		orders, err := op.GetOrders(req.Context(), userID)
		if err != nil {
			logger.Error("getOrdersList:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

		if len(orders) == 0 {
			logger.Info("getOrdersList:", zap.Error(err))
			http.Error(res, "No data", http.StatusNoContent)
			return
		}

		res.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(res)
		if err := encoder.Encode(orders); err != nil {
			logger.Error("getOrdersList:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}
	}
}

func GetBonusesAmount(bp BonusesProcessor, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		userID, ok := getUserIDFromContext(req.Context())
		if !ok {
			logger.Info("getBonusesAmount: unauthorized")
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		bonuses, err := bp.GetCurrentBonusesAmount(req.Context(), userID)
		if err != nil {
			logger.Error("getBonusesAmount:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}
		res.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(res)
		if err := encoder.Encode(bonuses); err != nil {
			logger.Error("getBonusesAmount:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}

	}
}

func WithdrawBonuses(bp BonusesProcessor, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		userID, ok := getUserIDFromContext(req.Context())
		if !ok {
			logger.Info("withdrawBonuses: unauthorized")
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var request models.APIUseBonusesRequest
		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&request); err != nil {
			logger.Info("withdrawBonuses:", zap.Error(err))
			http.Error(res, "Invalid request format", http.StatusInternalServerError)
			return
		}
		defer req.Body.Close()

		ok, err := isOrderNumberValid(request.OrderNumber)
		if !ok || err != nil {
			logger.Info("withdrawBonuses:", zap.Error(err))
			http.Error(res, "Incorrect order number format", http.StatusUnprocessableEntity)
			return
		}

		err = bp.UseBonuses(req.Context(), request, userID)
		if err != nil {
			if errors.Is(err, storage.ErrNotEnoughBonuses) {
				logger.Info("withdrawBonuses:", zap.Error(err))
				http.Error(res, "Not enough bonuses", http.StatusPaymentRequired)
				return
			} else {
				logger.Error("withdrawBonuses:", zap.Error(err))
				http.Error(res, "Internal error", http.StatusInternalServerError)
				return
			}
		}
	}
}

func GetWithdrawals(wp WithdrawalsProcessor, logger logger.Logger) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		userID, ok := getUserIDFromContext(req.Context())
		if !ok {
			logger.Info("getWithdrawals: unauthorized")
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		response, err := wp.GetWithdrawalsHistory(req.Context(), userID)
		if err != nil {
			if errors.Is(err, storage.ErrEmptyWithdrawalHistory) {
				logger.Info("getWithdrawals:", zap.Error(err))
				http.Error(res, "No withdrawals", http.StatusNoContent)
				return
			} else {
				logger.Error("getWithdrawals:", zap.Error(err))
				http.Error(res, "Internal error", http.StatusInternalServerError)
				return
			}
		}
		res.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(res)
		if err := encoder.Encode(response); err != nil {
			logger.Error("getWithdrawals:", zap.Error(err))
			http.Error(res, "Internal error", http.StatusInternalServerError)
			return
		}
	}
}
