package main

import (
	"encoding/json"
	"go-stripe/internal/cards"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type stripePayload struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type jsonResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Content string `json:"content,omitempty"`
	ID      int    `json:"id,omitempty"`
}

// get payment intent from stripe
func (app *application) GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload stripePayload

	// parse data
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		app.logger.Error("failed to decode body: ", zap.Error(err))
		return
	}

	amount, err := strconv.Atoi(payload.Amount)
	if err != nil {
		app.logger.Error("failed to convert amount: ", zap.Error(err))
		return
	}

	// initialize card
	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: payload.Currency,
	}

	ok := true
	pi, msg, err := card.Charge(payload.Currency, amount)
	if err != nil {
		app.logger.Error("failed process payment: ", zap.Error(err))
		ok = false
	}

	if ok {
		out, err := json.Marshal(pi)
		if err != nil {
			app.logger.Error("failed to decode payment intent: ", zap.Error(err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err = w.Write(out); err != nil {
			app.logger.Error("error writing response: ", zap.Error(err))
		}

	} else {
		j := jsonResponse{
			OK:      false,
			Message: msg,
			Content: "",
		}

		out, err := json.Marshal(j)
		if err != nil {
			app.logger.Error("failed to marshal json: ", zap.Error(err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err = w.Write(out); err != nil {
			app.logger.Error("error writing response: ", zap.Error(err))
		}
	}
}

// handle get widget by ID route
func (app *application) GetWidgetByID(w http.ResponseWriter, r *http.Request) {
	// get ID from url params
	id := chi.URLParam(r, "id")
	widgetID, err := strconv.Atoi(id)
	if err != nil {
		app.logger.Error("failed to get widget ID: ", zap.Error(err))
		return
	}

	// get widget from the database
	widget, err := app.DB.GetWidget(widgetID)
	if err != nil {
		app.logger.Error("failed to get widget from database: ", zap.Error(err))
		return
	}

	out, err := json.Marshal(widget)
	if err != nil {
		app.logger.Error("failed to get marshal json: ", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(out); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}
