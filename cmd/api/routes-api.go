package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// define routes
func (app *application) routes() http.Handler {
	mux := chi.NewRouter()

	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	mux.Post("/v"+app.version[0:1]+"/api/payment-intent", app.GetPaymentIntent)
	mux.Get("/v"+app.version[0:1]+"/api/widget/{id}", app.GetWidgetByID)

	mux.Post("/v"+app.version[0:1]+"/api/create-customer-subscribe", app.CreateCustomerSubscribe)

	mux.Post("/v"+app.version[0:1]+"/api/auth", app.CreateAuthToken)
	mux.Post("/v"+app.version[0:1]+"/api/is-authenticated", app.CheckAuth)
	mux.Post("/v"+app.version[0:1]+"/api/forgot-password", app.SendPasswordResetEmail)
	mux.Post("/v"+app.version[0:1]+"/api/reset-password", app.ResetPassword)

	mux.Route("/v"+app.version[0:1]+"/api/admin", func(mux chi.Router) {
		mux.Use(app.Auth)

		mux.Post("/virtual-terminal-succeeded", app.VirtualTerminalPaymentSucceeded)
		mux.Post("/all-sales", app.AllSales)
		mux.Post("/all-subscriptions", app.AllSubscriptions)

		mux.Post("/get-sale/{id}", app.GetSale)

		mux.Post("/refund", app.RefundCharge)
	})

	return mux
}
