package main

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/config"
	"github.com/vancho-go/gophermart/internal/app/handlers"
	"github.com/vancho-go/gophermart/internal/app/logger"
	"github.com/vancho-go/gophermart/internal/app/storage"
	"go.uber.org/zap"
	"log"
	"net/http"
	"time"
)

func periodicUpdateExecutor(ctx context.Context, interval time.Duration, accrualSystemAddress string, task func(context.Context, string, logger.Logger), logger logger.Logger) {
	for {
		task(ctx, accrualSystemAddress, logger)
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

const orderUpdaterPeriod = time.Millisecond * 500

func main() {
	configuration, err := config.BuildServer()
	if err != nil {
		log.Fatalf("error building server  configuration: %v", err)
	}

	err = auth.SetSecretKey(configuration.JWTSecretKey)
	if err != nil {
		log.Fatalf("failed setting jwt auth key: %v", err)
	}

	logger, err := logger.NewLogger("debug")

	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}

	dbInstance, err := storage.Initialize(configuration.DatabaseURI)
	if err != nil {
		logger.Fatal("error initialising database", zap.Error(err))
	}

	logger.Info("starting periodic update order numbers executor")
	ctx := context.Background()
	go periodicUpdateExecutor(ctx, orderUpdaterPeriod, configuration.AccrualSystemAddress, dbInstance.HandleOrderNumbers, logger)

	logger.Info("running server", zap.String("address", configuration.ServerRunAddress))
	r := chi.NewRouter()

	r.Route("/api/user", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Post("/register", handlers.RegisterUser(dbInstance, logger))
			r.Post("/login", handlers.AuthenticateUser(dbInstance, logger))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware)
			r.Post("/orders", handlers.AddOrder(dbInstance, logger))
			r.Get("/orders", handlers.GetOrdersList(dbInstance, logger))
			r.Get("/withdrawals", handlers.GetWithdrawals(dbInstance, logger))
		})

		r.Route("/balance", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware)
				r.Get("/", handlers.GetBonusesAmount(dbInstance, logger))
				r.Post("/withdraw", handlers.WithdrawBonuses(dbInstance, logger))
			})
		})
	})

	err = http.ListenAndServe(configuration.ServerRunAddress, r)
	if err != nil {
		logger.Fatal("error starting server", zap.Error(err))
	}
}
