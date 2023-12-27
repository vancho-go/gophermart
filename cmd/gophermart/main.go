package main

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/config"
	"github.com/vancho-go/gophermart/internal/app/handlers"
	"github.com/vancho-go/gophermart/internal/app/storage"
	"log"
	"net/http"
	"time"
)

func periodicUpdateExecutor(ctx context.Context, interval time.Duration, accrualSystemAddress string, task func(context.Context, string)) {
	for {
		task(ctx, accrualSystemAddress)
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func main() {
	configuration, err := config.BuildServer()
	if err != nil {
		log.Fatalf("error building server configuration: %s", err.Error())
	}

	//zapLogger, err := logger.NewLogger("debug")
	//_ = zapLogger
	//
	//if err != nil {
	//	log.Fatalf("failed to create logger: %s", err.Error())
	//}

	dbInstance, err := storage.Initialize(configuration.DatabaseURI)
	if err != nil {
		log.Fatalf("error initialising database: %s", err.Error())
	}

	ctx := context.Background()
	go periodicUpdateExecutor(ctx, time.Second, configuration.AccrualSystemAddress, dbInstance.HandleOrderNumbers)

	r := chi.NewRouter()

	r.Route("/api/user", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Post("/register", handlers.RegisterUser(dbInstance))
			r.Post("/login", handlers.AuthenticateUser(dbInstance))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware)
			// не обработана 400 ошибка
			r.Post("/orders", handlers.AddOrder(dbInstance))
			r.Get("/orders", handlers.GetOrdersList(dbInstance))
			r.Get("/withdrawals", handlers.GetWithdrawals(dbInstance))
		})

		r.Route("/balance", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware)
				r.Get("/", handlers.GetBonusesAmount(dbInstance))
				r.Post("/withdraw", handlers.WithdrawBonuses(dbInstance))
			})
		})
	})

	err = http.ListenAndServe(configuration.ServerRunAddress, r)
	if err != nil {
		panic(fmt.Errorf("error starting server: %w", err))
	}
}
