package models

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// type for database connection values
type DBModel struct {
	DB *sql.DB
}

// wrapper for all models
type Models struct {
	DB DBModel
}

// type for all widgets
type Widget struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	InventoryLevel int       `json:"inventory_level"`
	Price          int       `json:"price"`
	Image          string    `json:"image"`
	IsRecurring    bool      `json:"is_recurring"`
	PlanID         string    `json:"plan_id"`
	CreatedAt      time.Time `json:"-"`
	UpdatedAt      time.Time `json:"-"`
}

// type for all orders
type Order struct {
	ID            int       `json:"id"`
	WidgetID      int       `json:"widget_id"`
	TransactionID int       `json:"transaction_id"`
	CustomerID    int       `json:"customer_id"`
	StatusID      int       `json:"status_id"`
	Quantity      int       `json:"quantity"`
	Amount        int       `json:"amount"`
	CreatedAt     time.Time `json:"-"`
	UpdatedAt     time.Time `json:"-"`
}

// type for all order statuses
type Status struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// type for all transaction statuses
type TransactionStatus struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// type for all transactions
type Transaction struct {
	ID                  int       `json:"id"`
	Amount              int       `json:"amount"`
	Currency            string    `json:"currency"`
	LastFour            string    `json:"last_four"`
	ExpiryMonth         int       `json:"expiry_month"`
	ExpiryYear          int       `json:"expiry_year"`
	BankReturnCode      string    `json:"bank_return_code"`
	TransactionStatusID int       `json:"transaction_status_id"`
	PaymentIntent       string    `json:"payment_intent"`
	PaymentMethod       string    `json:"payment_method"`
	CreatedAt           time.Time `json:"-"`
	UpdatedAt           time.Time `json:"-"`
}

// type for all users
type User struct {
	ID        int       `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// type for all customers
type Customer struct {
	ID        int       `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// returns a model type with database connection pool
func NewModels(db *sql.DB) Models {
	return Models{
		DB: DBModel{
			DB: db,
		},
	}
}

// get widget by ID from database
func (m *DBModel) GetWidget(id int) (Widget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var widget Widget

	row := m.DB.QueryRowContext(ctx, `
		SELECT
			id, name, description, inventory_level, price, coalesce(image, ''), is_recurring, plan_id, created_at, updated_at
		FROM
			widgets
		WHERE id = ?
		`, id)
	if err := row.Scan(
		&widget.ID,
		&widget.Name,
		&widget.Description,
		&widget.InventoryLevel,
		&widget.Price,
		&widget.Image,
		&widget.IsRecurring,
		&widget.PlanID,
		&widget.CreatedAt,
		&widget.UpdatedAt,
	); err != nil {
		fmt.Println(err)
		return widget, err
	}

	return widget, nil
}

// inserts a new tx and returns its id
func (m *DBModel) InsertTransaction(tx Transaction) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		INSERT INTO transactions
			(amount, currency, last_four, expiry_month, expiry_year, bank_return_code, transaction_status_id, payment_intent, payment_method, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.DB.ExecContext(ctx, query,
		tx.Amount,
		tx.Currency,
		tx.LastFour,
		tx.ExpiryMonth,
		tx.ExpiryYear,
		tx.BankReturnCode,
		tx.TransactionStatusID,
		tx.PaymentIntent,
		tx.PaymentMethod,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	return int(id), nil
}

// inserts a new order and returns its id
func (m *DBModel) InsertOrder(order Order) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		INSERT INTO orders
			(widget_id, transaction_id, status_id, quantity, customer_id, amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.DB.ExecContext(ctx, query,
		order.WidgetID,
		order.TransactionID,
		order.StatusID,
		order.Quantity,
		order.CustomerID,
		order.Amount,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	return int(id), nil
}

// inserts a new customer and returns its id
func (m *DBModel) InsertCustomer(c Customer) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		INSERT INTO customers
			(first_name, last_name, email, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := m.DB.ExecContext(ctx, query,
		c.FirstName,
		c.LastName,
		c.Email,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	return int(id), nil
}