package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/vancho-go/gophermart/internal/app/config"
	"github.com/vancho-go/gophermart/internal/app/logger"
	"log"
	"net/http"
)

func main() {
	configuration, err := config.BuildServer()
	if err != nil {
		panic(fmt.Errorf("error building server configuration: %w", err))
	}

	zapLogger, err := logger.NewZapLogger("debug")

	if err != nil {
		log.Fatalf("Failed to create logger: %s", err.Error())
	}

	r := chi.NewRouter()

	r.Route("/api/user", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Post("/register", nil)
			r.Post("/login", nil)
			r.Post("/orders", nil)
			r.Get("/orders", nil)
			r.Get("/withdrawals", nil)
		})

		r.Route("/balance", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Get("/", nil)
				r.Post("/withdraw", nil)
			})
		})
	})

	err = http.ListenAndServe(configuration.ServerRunAddress, r)
	if err != nil {
		panic(fmt.Errorf("error starting server: %w", err))
	}
}
