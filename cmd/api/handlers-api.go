package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-stripe/internal/cards"
	"go-stripe/internal/encryption"
	"go-stripe/internal/models"
	"go-stripe/internal/urlsigner"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type stripePayload struct {
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	PaymentIntent string `json:"payment_intent"`
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

type Invoice struct {
	ID        int       `json:"id"`
	Quantity  int       `json:"quantity"`
	Amount    int       `json:"amount"`
	Product   string    `json:"product"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
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
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
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
		app.logger.Error("failed to decode json body: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	// initialize card
	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: data.Currency,
	}

	stripeCustomer, msg, err := card.CreateCustomer(data.PaymentMethod, data.Email)
	if err != nil {
		app.logger.Error("failed to create customer: ", err)
		if err = app.badRequest(w, r, errors.New(msg)); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	subscription, err := card.SubscribeToPlan(stripeCustomer, data.Plan, data.Email, data.LastFour, "")
	if err != nil {
		app.logger.Error("failed to subscribe to plan: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	productID, err := strconv.Atoi(data.ProductID)
	if err != nil {
		app.logger.Error("failed to convert product id: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	customerID, err := app.SaveCustomer(data.FirstName, data.LastName, data.Email)
	if err != nil {
		app.logger.Error("failed to save customer: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	amount, err := strconv.Atoi(data.Amount)
	if err != nil {
		app.logger.Error("failed to convert amount: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	tx := models.Transaction{
		Amount:              amount,
		Currency:            data.Currency,
		LastFour:            data.LastFour,
		ExpiryMonth:         data.ExpiryMonth,
		ExpiryYear:          data.ExpiryYear,
		TransactionStatusID: 2,
		PaymentMethod:       data.PaymentMethod,
		PaymentIntent:       subscription.ID,
	}

	txID, err := app.SaveTransaction(tx)
	if err != nil {
		app.logger.Error("failed to save transaction: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
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

	orderID, err := app.SaveOrder(order)
	if err != nil {
		app.logger.Error("failed to save order: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error("failed to write response: ", err)
		}
		return
	}

	inv := Invoice{
		ID: orderID,
		// TODO:
		Amount: 2000,
		// TODO: get from database
		Product:   "Bronze Plan",
		Quantity:  order.Quantity,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		CreatedAt: time.Now(),
	}

	err = app.callInvoiceMicro(inv)
	if err != nil {
		app.logger.Error("failed to call invoice microservice: ", zap.Error(err))
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	resp.Error = false
	resp.Message = "Transaction Successful: " + fmt.Sprint(inv)

	if err = app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("failed to write response: ", err)
	}
}

func (app *application) callInvoiceMicro(inv Invoice) error {
	// TODO: add to env vars
	url := "http://localhost:4002/v1/invoice/create-and-send"

	out, err := json.Marshal(inv)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(out))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
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

func (app *application) VirtualTerminalPaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	var txData struct {
		PaymentAmount   int    `json:"amount"`
		PaymentCurrency string `json:"currency"`
		FirstName       string `json:"first_name"`
		LastName        string `json:"last_name"`
		Email           string `json:"email"`
		PaymentIntent   string `json:"payment_intent"`
		PaymentMethod   string `json:"payment_method"`
		BankReturnCode  string `json:"bank_return_code"`
		ExpiryMonth     int    `json:"expiry_month"`
		ExpiryYear      int    `json:"expiry_year"`
		LastFour        string `json:"last_four"`
	}

	err := app.readJSON(w, r, &txData)
	if err != nil {
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	card := cards.Card{
		Secret: app.config.stripe.secret,
		Key:    app.config.stripe.key,
	}

	pi, err := card.RetrievePaymentIntent(txData.PaymentIntent)
	if err != nil {
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	pm, err := card.GetPaymentMethod(txData.PaymentMethod)
	if err != nil {
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	txData.LastFour = pm.Card.Last4
	txData.ExpiryMonth = int(pm.Card.ExpMonth)
	txData.ExpiryYear = int(pm.Card.ExpYear)

	tx := models.Transaction{
		Amount:              txData.PaymentAmount,
		Currency:            txData.PaymentCurrency,
		LastFour:            txData.LastFour,
		ExpiryMonth:         txData.ExpiryMonth,
		ExpiryYear:          txData.ExpiryYear,
		BankReturnCode:      pi.Charges.Data[0].ID,
		TransactionStatusID: 2,
		PaymentIntent:       txData.PaymentIntent,
		PaymentMethod:       txData.PaymentMethod,
	}

	_, err = app.SaveTransaction(tx)
	if err != nil {
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if err = app.writeJson(w, http.StatusOK, tx); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	var userInput struct {
		Email string `json:"email"`
	}

	err := app.readJSON(w, r, &userInput)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var payload struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	_, err = app.DB.GetUserByEmail(userInput.Email)
	if err != nil {
		payload.Error = true
		payload.Message = "User not found."
		if err := app.writeJson(w, http.StatusAccepted, payload); err != nil {
			app.logger.Error("error writing response: ", zap.Error(err))
		}
		return
	}

	link := fmt.Sprintf("%s/reset-password?email=%s", app.config.frontend, userInput.Email)
	sign := urlsigner.Signer{
		Secret: []byte(app.config.secretKey),
	}

	signedLink := sign.GenerateTokenFromString(link)

	var data struct {
		Link string
	}

	data.Link = signedLink

	err = app.SendMail("info@widgets.com", userInput.Email, "Password Reset Request", "password-reset", data)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	payload.Error = false
	payload.Message = "Password reset email sent!"

	if err := app.writeJson(w, http.StatusCreated, payload); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) ResetPassword(w http.ResponseWriter, r *http.Request) {
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

	encryptor := encryption.Encryption{
		Key: []byte(app.config.secretKey),
	}

	decryptedEmail, err := encryptor.Decrypt(userInput.Email)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	user, err := app.DB.GetUserByEmail(decryptedEmail)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(userInput.Password), 12)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	err = app.DB.UpdatePasswordForUser(user, string(newHash))
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var payload struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	payload.Error = false
	payload.Message = "Password changed successfully!"

	if err := app.writeJson(w, http.StatusCreated, payload); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) AllSales(w http.ResponseWriter, r *http.Request) {
	var userInput struct {
		PageSize    int `json:"page_size"`
		CurrentPage int `json:"page"`
	}

	err := app.readJSON(w, r, &userInput)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	allSales, lastPage, totalRecords, err := app.DB.GetAllOrders(userInput.PageSize, userInput.CurrentPage)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		CurrentPage  int             `json:"current_page"`
		PageSize     int             `json:"page_size"`
		LastPage     int             `json:"last_page"`
		TotalRecords int             `json:"total_records"`
		Orders       []*models.Order `json:"orders"`
	}

	resp.CurrentPage = userInput.CurrentPage
	resp.PageSize = userInput.PageSize
	resp.LastPage = lastPage
	resp.TotalRecords = totalRecords
	resp.Orders = allSales

	if err := app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) AllSubscriptions(w http.ResponseWriter, r *http.Request) {
	var userInput struct {
		PageSize    int `json:"page_size"`
		CurrentPage int `json:"page"`
	}

	err := app.readJSON(w, r, &userInput)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	allSubscriptions, lastPage, totalRecords, err := app.DB.GetAllSubscriptions(userInput.PageSize, userInput.CurrentPage)

	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		CurrentPage  int             `json:"current_page"`
		PageSize     int             `json:"page_size"`
		LastPage     int             `json:"last_page"`
		TotalRecords int             `json:"total_records"`
		Orders       []*models.Order `json:"orders"`
	}

	resp.CurrentPage = userInput.CurrentPage
	resp.PageSize = userInput.PageSize
	resp.LastPage = lastPage
	resp.TotalRecords = totalRecords
	resp.Orders = allSubscriptions

	if err := app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) GetSale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orderID, err := strconv.Atoi(id)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	order, err := app.DB.GetOrderByID(orderID)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if err := app.writeJson(w, http.StatusOK, order); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) RefundCharge(w http.ResponseWriter, r *http.Request) {
	var chargeToRefund struct {
		ID            int    `json:"id"`
		PaymentIntent string `json:"payment_intent"`
		Amount        int    `json:"amount"`
		Currency      string `json:"currency"`
	}

	err := app.readJSON(w, r, &chargeToRefund)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	order, err := app.DB.GetOrderByID(chargeToRefund.ID)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if order.Amount != chargeToRefund.Amount {
		if err = app.badRequest(w, r, errors.New("amounts do not match")); err != nil {
			app.logger.Error(err)
		}
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: chargeToRefund.Currency,
	}

	err = card.Refund(chargeToRefund.PaymentIntent, chargeToRefund.Amount)
	if err != nil {
		app.logger.Error("error refunding payment: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if err = app.DB.UpdateOrderStatus(chargeToRefund.ID, 2); err != nil {
		errResp := errors.New("the charge was refunded, but the database could not be updated")
		app.logger.Error(errResp)
		if err = app.badRequest(w, r, errResp); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	resp.Error = false
	resp.Message = "Charge refunded"

	if err := app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	var subToCancel struct {
		ID            int    `json:"id"`
		PaymentIntent string `json:"payment_intent"`
		Currency      string `json:"currency"`
	}

	err := app.readJSON(w, r, &subToCancel)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: subToCancel.Currency,
	}

	if err = card.CancelSubscription(subToCancel.PaymentIntent); err != nil {
		app.logger.Error("error cancelling subscription: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if err = app.DB.UpdateOrderStatus(subToCancel.ID, 3); err != nil {
		errResp := errors.New("the subscription was cancelled, but the database could not be updated")
		app.logger.Error(errResp)
		if err = app.badRequest(w, r, errResp); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	resp.Error = false
	resp.Message = "Subscription cancelled"

	if err := app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}

}

func (app *application) AllUsers(w http.ResponseWriter, r *http.Request) {
	allUsers, err := app.DB.GetAllUsers()
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if err := app.writeJson(w, http.StatusOK, allUsers); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}

}

func (app *application) OneUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	user, err := app.DB.GetUserByID(userID)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	if err := app.writeJson(w, http.StatusOK, user); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}

}

func (app *application) EditUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var user models.User

	err = app.readJSON(w, r, &user)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	if userID > 0 {
		err = app.DB.EditUser(user)
		if err != nil {
			app.logger.Error(err)
			if err = app.badRequest(w, r, err); err != nil {
				app.logger.Error(err)
			}
			return
		}

		if user.Password != "" {
			newHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
			if err != nil {
				app.logger.Error(err)
				if err = app.badRequest(w, r, err); err != nil {
					app.logger.Error(err)
				}
				return
			}

			if err = app.DB.UpdatePasswordForUser(user, string(newHash)); err != nil {
				app.logger.Error(err)
				if err = app.badRequest(w, r, err); err != nil {
					app.logger.Error(err)
				}
				return
			}
		}

		resp.Message = "User updated added successfully"
	} else {
		newHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
		if err != nil {
			app.logger.Error(err)
			if err = app.badRequest(w, r, err); err != nil {
				app.logger.Error(err)
			}
			return
		}

		if err = app.DB.AddUser(user, string(newHash)); err != nil {
			app.logger.Error(err)
			if err = app.badRequest(w, r, err); err != nil {
				app.logger.Error(err)
			}
			return
		}

		resp.Message = "New user added successfully"
	}

	resp.Error = false

	if err := app.writeJson(w, http.StatusOK, user); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	err = app.DB.DeleteUser(userID)
	if err != nil {
		app.logger.Error(err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	resp.Error = false
	resp.Message = "User deleted successfully"

	if err := app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}

}
