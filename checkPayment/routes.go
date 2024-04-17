package main

import (
	authmiddleware "checkPayment/auth"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Handlers the routing part, returns Handler to the main.go
func (app *Config) routes() http.Handler {
	mux := chi.NewRouter()

	// Specify who is allowed to connect
	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-TOKEN"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	//To check if service up or not
	mux.Use(middleware.Heartbeat("/ping"))

	//JWT middleware
	mux.Use(authmiddleware.JWTMiddleware)

	//Add route at root level
	mux.Post("/checkPayment", app.checkPayment)
	mux.Post("/AbouttoCheckout",app.AbouttoCheckout)

	return mux
}
