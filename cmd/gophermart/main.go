package main

import "github.com/go-chi/chi/v5"

func main() {
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
}
