package main

import (
	"go-stripe/internal/models"
	"net/http"

	"go.uber.org/zap"
)

func (app *application) VirtualTerminal(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "terminal", &templateData{}, "stripe-js"); err != nil {
		app.logger.Fatal(err)
	}
}

func (app *application) PaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		app.logger.Error("failed to parse form: ", zap.Error(err))
		return
	}

	data := map[string]any{
		"cardholder": r.Form.Get("cardholder_name"),
		"email":      r.Form.Get("cardholder_email"),
		"pi":         r.Form.Get("payment_intent"),
		"pm":         r.Form.Get("payment_method"),
		"pa":         r.Form.Get("payment_amount"),
		"pc":         r.Form.Get("payment_currency"),
	}

	if err := app.renderTemplate(w, r, "succeeded", &templateData{Data: data}); err != nil {
		app.logger.Fatal(err)
	}
}

func (app *application) ChargeOnce(w http.ResponseWriter, r *http.Request) {
	widget := models.Widget{
		ID:             1,
		Name:           "Custom Widget",
		Description:    "A very nice widget",
		InventoryLevel: 10,
		Price:          1000,
	}

	data := map[string]any{
		"widget": widget,
	}

	if err := app.renderTemplate(w, r, "charge-once", &templateData{Data: data}, "stripe-js"); err != nil {
		app.logger.Fatal(err)
	}
}
