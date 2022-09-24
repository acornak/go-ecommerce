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
	CreatedAt      time.Time `json:"-"`
	UpdatedAt      time.Time `json:"-"`
}

// returns a model type with database connection pool
func NewModels(db *sql.DB) Models {
	return Models{
		DB: DBModel{
			DB: db,
		},
	}
}

func (m *DBModel) GetWidget(id int) (Widget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var widget Widget

	row := m.DB.QueryRowContext(ctx, "SELECT id, name FROM widgets WHERE id = ?", id)
	if err := row.Scan(&widget.ID, &widget.Name); err != nil {
		fmt.Println(err)
		return widget, err
	}

	return widget, nil
}
