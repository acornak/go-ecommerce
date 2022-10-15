package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-stripe/internal/cards"
	"go-stripe/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type stripePayload struct {
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Email         string `json:"email"`
	CardBrand     string `json:"card_brand"`
	ExpiryMonth   int    `json:"exp_month"`
	ExpiryYear    int    `json:"exp_year"`
	LastFour      string `json:"last_four"`
	Plan          string `json:"plan"`
	ProductID     string `json:"product_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
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

func (app *application) CreateCustomerSubscribe(w http.ResponseWriter, r *http.Request) {
	var data stripePayload

	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		app.logger.Error("failed to decode body: ", zap.Error(err))
		return
	}

	// initialize card
	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: data.Currency,
	}

	ok := true
	txMsg := "Transaction successful"

	stripeCustomer, msg, err := card.CreateCustomer(data.PaymentMethod, data.Email)
	if err != nil {
		app.logger.Error("failed to create new customer: ", zap.Error(err))
		ok = false
		txMsg = msg
	}

	if ok {
		_, err := card.SubsctibeToPlan(stripeCustomer, data.Plan, data.Email, data.LastFour, "")
		if err != nil {
			txMsg = "failed to subscribe customer to a plan"
			app.logger.Error(txMsg, ": ", zap.Error(err))
			ok = false
		}
	}

	if ok {
		func() {
			productID, err := strconv.Atoi(data.ProductID)
			if err != nil {
				txMsg = "failed to convert productID"
				app.logger.Error(txMsg, ": ", zap.Error(err))
				ok = false
				return
			}

			customerID, err := app.SaveCustomer(data.FirstName, data.LastName, data.Email)
			if err != nil {
				txMsg = "failed to convert productID: "
				app.logger.Error(txMsg, ": ", zap.Error(err))
				ok = false
				return
			}

			amount, err := strconv.Atoi(data.Amount)
			if err != nil {
				txMsg = "failed to convert amount"
				app.logger.Error(txMsg, ": ", zap.Error(err))
				ok = false
				return
			}

			tx := models.Transaction{
				Amount:              amount,
				Currency:            data.Currency,
				LastFour:            data.LastFour,
				ExpiryMonth:         data.ExpiryMonth,
				ExpiryYear:          data.ExpiryYear,
				TransactionStatusID: 2,
			}

			txID, err := app.SaveTransaction(tx)
			if err != nil {
				txMsg = "failed to get transaction ID"
				app.logger.Error(txMsg, ": ", zap.Error(err))
				ok = false
				return
			}

			order := models.Order{
				WidgetID:      productID,
				TransactionID: txID,
				CustomerID:    customerID,
				StatusID:      1,
				Quantity:      1,
				Amount:        amount,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			_, err = app.SaveOrder(order)
			if err != nil {
				txMsg = "failed to save order"
				app.logger.Error(txMsg, ": ", zap.Error(err))
				ok = false
				return
			}
		}()
	}

	resp := jsonResponse{
		OK:      ok,
		Message: txMsg,
		Content: "",
	}

	// TODO: use writeJson
	out, err := json.Marshal(resp)
	if err != nil {
		app.logger.Error("failed to get marshal json: ", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(out); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

// create a new customer
func (app *application) SaveCustomer(firstName, lastName, email string) (int, error) {
	customer := models.Customer{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
	}

	id, err := app.DB.InsertCustomer(customer)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// saves a transaction and returns its ID
func (app *application) SaveTransaction(tx models.Transaction) (int, error) {
	id, err := app.DB.InsertTransaction(tx)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// saves an order and returns its ID
func (app *application) SaveOrder(order models.Order) (int, error) {
	id, err := app.DB.InsertOrder(order)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// handler for /auth route
func (app *application) CreateAuthToken(w http.ResponseWriter, r *http.Request) {
	var userInput struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &userInput)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	user, err := app.DB.GetUserByEmail(userInput.Email)
	if err != nil {
		if err = app.invalidCredentials(w); err != nil {
			app.logger.Error(err)
		}
		return
	}

	validPassword, err := app.passwordMatches(user.Password, userInput.Password)
	if err != nil || !validPassword {
		if err = app.invalidCredentials(w); err != nil {
			app.logger.Error(err)
		}
		return
	}

	token, err := models.GenerateToken(user.ID, 24*time.Hour, models.ScopeAuthentication)
	if err != nil {
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	err = app.DB.InsertToken(token, user)
	if err != nil {
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var payload struct {
		Error   bool          `json:"error"`
		Message string        `json:"message"`
		Token   *models.Token `json:"auth_token"`
	}

	payload.Error = false
	payload.Message = fmt.Sprintf("token for %s created", userInput.Email)
	payload.Token = token

	if err = app.writeJson(w, http.StatusOK, payload); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}

}

func (app *application) authenticateToken(r *http.Request) (*models.User, error) {
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader == "" {
		return nil, errors.New("no authorization header received")
	}

	headerParts := strings.Split(authorizationHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return nil, errors.New("no authorization header received")
	}

	token := headerParts[1]
	if len(token) != 26 {
		return nil, errors.New("invalid authentication token")
	}

	user, err := app.DB.GetUserForToken(token)
	if err != nil {
		return nil, errors.New("invalid authentication token")
	}

	return user, nil
}

func (app *application) CheckAuth(w http.ResponseWriter, r *http.Request) {
	user, err := app.authenticateToken(r)
	if err != nil {
		if err = app.invalidCredentials(w); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var payload struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	payload.Error = false
	payload.Message = fmt.Sprintf("authenticated user %s", user.Email)

	if err = app.writeJson(w, http.StatusOK, payload); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}
