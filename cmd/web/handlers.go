package main

import (
	"net/http"

	"go.uber.org/zap"
)

func (app *application) VirtualTerminal(w http.ResponseWriter, r *http.Request) {
	stringMap := map[string]string{
		"publishable_key": app.config.stripe.key,
	}

	if err := app.renderTemplate(w, r, "terminal", &templateData{StringMap: stringMap}); err != nil {
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
