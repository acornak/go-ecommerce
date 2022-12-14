package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-stripe/internal/cards"
	"go-stripe/internal/encryption"
	"go-stripe/internal/models"
	"go-stripe/internal/urlsigner"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type TransactionData struct {
	FirstName       string
	LastName        string
	Email           string
	PaymentIntentID string
	PaymentMethodID string
	PaymentAmount   int
	PaymentCurrency string
	LastFour        string
	ExpiryMonth     int
	ExpiryYear      int
	BankReturnCode  string
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

// handler for homepage
func (app *application) Home(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "home", &templateData{}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
	}
}

// handler for virtual terminal page
func (app *application) VirtualTerminal(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "terminal", &templateData{}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
	}
}

// get transaction data from strie and post
func (app *application) GetTransactionData(r *http.Request) (TransactionData, error) {
	var txData TransactionData

	// get data from the form
	if err := r.ParseForm(); err != nil {
		app.logger.Error("failed to parse form: ", zap.Error(err))
		return txData, err
	}

	firstName := r.Form.Get("first_name")
	lastName := r.Form.Get("last_name")
	email := r.Form.Get("cardholder_email")
	currency := r.Form.Get("payment_currency")
	paymentIntent := r.Form.Get("payment_intent")
	paymentMethod := r.Form.Get("payment_method")

	// initialize card and get payment data
	card := cards.Card{
		Secret: app.config.stripe.secret,
		Key:    app.config.stripe.key,
	}

	pi, err := card.RetrievePaymentIntent(paymentIntent)
	if err != nil {
		app.logger.Error("failed to retrieve payment intent: ", zap.Error(err))
		return txData, err
	}

	pm, err := card.GetPaymentMethod(paymentMethod)
	if err != nil {
		app.logger.Error("failed to get payment method: ", zap.Error(err))
		return txData, err
	}

	// create a new transaction
	amount, err := strconv.Atoi(r.Form.Get("payment_amount"))
	if err != nil {
		app.logger.Error("failed to parse payment amount: ", zap.Error(err))
		return txData, err
	}

	txData = TransactionData{
		FirstName:       firstName,
		LastName:        lastName,
		Email:           email,
		PaymentIntentID: paymentIntent,
		PaymentMethodID: paymentMethod,
		PaymentAmount:   amount,
		PaymentCurrency: currency,
		LastFour:        pm.Card.Last4,
		ExpiryMonth:     int(pm.Card.ExpMonth),
		ExpiryYear:      int(pm.Card.ExpYear),
		BankReturnCode:  pi.Charges.Data[0].ID,
	}

	return txData, nil

}

// handler for payment succeeded page
func (app *application) PaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	// get data from the form
	if err := r.ParseForm(); err != nil {
		app.logger.Error("failed to parse form: ", zap.Error(err))
		return
	}

	// create a new order
	widgetID, err := strconv.Atoi(r.Form.Get("product_id"))
	if err != nil {
		app.logger.Error("failed to parse widget id: ", zap.Error(err))
		return
	}

	txData, err := app.GetTransactionData(r)
	if err != nil {
		app.logger.Error("failed to get transaction data: ", zap.Error(err))
		return
	}

	// create a new customer
	customerID, err := app.SaveCustomer(txData.FirstName, txData.LastName, txData.Email)
	if err != nil {
		app.logger.Error("failed to insert a new customer: ", zap.Error(err))
		return
	}

	tx := models.Transaction{
		Amount:              txData.PaymentAmount,
		Currency:            txData.PaymentCurrency,
		LastFour:            txData.LastFour,
		ExpiryMonth:         txData.ExpiryMonth,
		ExpiryYear:          txData.ExpiryYear,
		BankReturnCode:      txData.BankReturnCode,
		TransactionStatusID: 2,
		PaymentIntent:       txData.PaymentIntentID,
		PaymentMethod:       txData.PaymentMethodID,
	}

	txID, err := app.SaveTransaction(tx)
	if err != nil {
		app.logger.Error("failed to insert a new transaction: ", zap.Error(err))
		return
	}

	order := models.Order{
		WidgetID:      widgetID,
		TransactionID: txID,
		CustomerID:    customerID,
		StatusID:      1,
		Quantity:      1,
		Amount:        txData.PaymentAmount,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	orderID, err := app.SaveOrder(order)
	if err != nil {
		app.logger.Error("failed to save order: ", zap.Error(err))
		return
	}

	// create and send invoice
	inv := Invoice{
		ID:     orderID,
		Amount: order.Amount,
		// TODO: get from database
		Product:   "Widget",
		Quantity:  order.Quantity,
		FirstName: txData.FirstName,
		LastName:  txData.LastName,
		Email:     txData.Email,
		CreatedAt: time.Now(),
	}

	err = app.callInvoiceMicro(inv)
	if err != nil {
		app.logger.Error("failed to call invoice microservice: ", zap.Error(err))
	}

	// write data to session and redirect user to receipt page
	app.Session.Put(r.Context(), "receipt", txData)
	http.Redirect(w, r, "/receipt", http.StatusSeeOther)
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

// handler for receipt page
func (app *application) Receipt(w http.ResponseWriter, r *http.Request) {
	exists := app.Session.Exists(r.Context(), "receipt")
	if !exists {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tx := app.Session.Get(r.Context(), "receipt").(TransactionData)
	data := map[string]any{
		"tx": tx,
	}
	app.Session.Remove(r.Context(), "receipt")

	if err := app.renderTemplate(w, r, "receipt", &templateData{Data: data}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

// saves a customer and returns its ID
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

// handler for charge once page
func (app *application) ChargeOnce(w http.ResponseWriter, r *http.Request) {
	// get widget ID from url
	id := chi.URLParam(r, "id")
	widgetID, err := strconv.Atoi(id)
	if err != nil {
		app.logger.Error("failed to get widget ID: ", zap.Error(err))
		return
	}

	// get widget data from database
	widget, err := app.DB.GetWidget(widgetID)
	if err != nil {
		app.logger.Error("failed to get widget from database: ", zap.Error(err))
		return
	}

	data := map[string]any{
		"widget": widget,
	}

	if err := app.renderTemplate(w, r, "charge-once", &templateData{Data: data}, "stripe-js"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

// handler for bronze plan
func (app *application) BronzePlan(w http.ResponseWriter, r *http.Request) {
	// get only 1 plan at the moment
	widget, err := app.DB.GetWidget(2)
	if err != nil {
		app.logger.Error("failed to get widget from database: ", zap.Error(err))
		return
	}

	data := map[string]any{
		"widget": widget,
	}

	if err := app.renderTemplate(w, r, "bronze-plan", &templateData{Data: data}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

// handler for bronze plan receipt
func (app *application) BronzePlanReceipt(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "receipt-plan", &templateData{}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

// displays the login page
func (app *application) LoginPage(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "login", &templateData{}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

// handles session after login
func (app *application) PostLoginPage(w http.ResponseWriter, r *http.Request) {
	if err := app.Session.RenewToken(r.Context()); err != nil {
		app.logger.Error("failed to renew token: ", err)
		return
	}

	if err := r.ParseForm(); err != nil {
		app.logger.Error("failed to parse form: ", err)
		return
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	id, err := app.DB.Authenticate(email, password)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.Session.Put(r.Context(), "userID", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handles logout
func (app *application) Logout(w http.ResponseWriter, r *http.Request) {
	if err := app.Session.Destroy(r.Context()); err != nil {
		app.logger.Error("failed to destroy session: ", err)
		return
	}
	if err := app.Session.RenewToken(r.Context()); err != nil {
		app.logger.Error("failed to renew token: ", err)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// forgot password handler
func (app *application) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "forgot-password", &templateData{}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

// reset password handler
func (app *application) ShowResetPassword(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	theURL := r.RequestURI
	testURL := fmt.Sprintf("%s%s", app.config.frontend, theURL)

	signer := urlsigner.Signer{
		Secret: []byte(app.config.secretKey),
	}

	valid := signer.VerityToken(testURL)
	if !valid {
		app.logger.Error("invalid url - tampering detected")
		if _, err := w.Write([]byte("invalid")); err != nil {
			app.logger.Error("error writing response: ", zap.Error(err))
		}
	}

	expired := signer.Expired(testURL, 5)
	if expired {
		app.logger.Error("invalid url - tampering detected")
		if _, err := w.Write([]byte("link expired")); err != nil {
			app.logger.Error("error writing response: ", zap.Error(err))
		}
	}

	encryptor := encryption.Encryption{
		Key: []byte(app.config.secretKey),
	}

	encryptedEmail, err := encryptor.Encrypt(email)
	if err != nil {
		app.logger.Error("encryption failed")
		return
	}

	data := map[string]any{
		"email": encryptedEmail,
	}

	if err := app.renderTemplate(w, r, "reset-password", &templateData{Data: data}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

func (app *application) AllSales(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "all-sales", &templateData{}, "format-currency"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

func (app *application) AllSubscriptions(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "all-subscriptions", &templateData{}, "format-currency"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

func (app *application) ShowSale(w http.ResponseWriter, r *http.Request) {
	stringMap := map[string]string{
		"title":        "Sale",
		"cancel":       "/admin/all-sales",
		"refund-url":   "/v1/api/admin/refund",
		"refund-btn":   "Refund Order",
		"alert-text":   "Refunded",
		"message-text": "Charge refunded",
	}

	if err := app.renderTemplate(w, r, "sale", &templateData{StringMap: stringMap}, "format-currency"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

func (app *application) ShowSubscription(w http.ResponseWriter, r *http.Request) {
	stringMap := map[string]string{
		"title":        "Subscription",
		"cancel":       "/admin/all-subscriptions",
		"refund-url":   "/v1/api/admin/cancel-subscription",
		"refund-btn":   "Cancel Subscription",
		"alert-text":   "Cancelled",
		"message-text": "Subscription cancelled",
	}

	if err := app.renderTemplate(w, r, "sale", &templateData{StringMap: stringMap}, "format-currency"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

func (app *application) AllUsers(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "all-users", &templateData{}, "format-currency"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}

func (app *application) OneUser(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "one-user", &templateData{}, "format-currency"); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
}
