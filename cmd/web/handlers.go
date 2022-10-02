package main

import (
	"go-stripe/internal/cards"
	"go-stripe/internal/models"
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

// handler for homepage
func (app *application) Home(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "home", &templateData{}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
	}
}

// handler for virtual terminal page
func (app *application) VirtualTerminal(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "terminal", &templateData{}, "stripe-js"); err != nil {
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

	if _, err = app.SaveOrder(order); err != nil {
		app.logger.Error("failed to save order: ", zap.Error(err))
		return
	}

	// write data to session and redirect user to receipt page
	app.Session.Put(r.Context(), "receipt", txData)
	http.Redirect(w, r, "/receipt", http.StatusSeeOther)
}

// handler for payment succeeded page for virtual terminal transactions
func (app *application) VirtualTerminalPaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	txData, err := app.GetTransactionData(r)
	if err != nil {
		app.logger.Error("failed to get transaction data: ", zap.Error(err))
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

	_, err = app.SaveTransaction(tx)
	if err != nil {
		app.logger.Error("failed to insert a new transaction: ", zap.Error(err))
		return
	}

	// write data to session and redirect user to receipt page
	app.Session.Put(r.Context(), "virtual-terminal-receipt", txData)
	http.Redirect(w, r, "/virtual-terminal-receipt", http.StatusSeeOther)
}

// handler for receipt page for virtual terminal
func (app *application) VirtualTerminalReceipt(w http.ResponseWriter, r *http.Request) {
	exists := app.Session.Exists(r.Context(), "virtual-terminal-receipt")
	if !exists {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tx := app.Session.Get(r.Context(), "virtual-terminal-receipt").(TransactionData)
	data := map[string]any{
		"tx": tx,
	}
	app.Session.Remove(r.Context(), "virtual-terminal-receipt")

	if err := app.renderTemplate(w, r, "virtual-terminal-receipt", &templateData{Data: data}); err != nil {
		app.logger.Error("unable to render template: ", zap.Error(err))
		return
	}
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
