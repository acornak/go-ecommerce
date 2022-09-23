package main

import (
	"encoding/json"
	"go-stripe/internal/cards"
	"net/http"
	"strconv"

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

func (app *application) GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload stripePayload

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
		out, err := json.MarshalIndent(pi, "", "    ")
		if err != nil {
			app.logger.Error("failed to decode payment intent: ", zap.Error(err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)

	} else {
		j := jsonResponse{
			OK:      false,
			Message: msg,
			Content: "",
		}

		out, err := json.MarshalIndent(j, "", "    ")
		if err != nil {
			app.logger.Error("failed to marshal json: ", zap.Error(err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}

}