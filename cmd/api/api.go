package main

import (
	"flag"
	"fmt"
	"go-stripe/internal/driver"
	"go-stripe/internal/models"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn string
	}
	stripe struct {
		secret string
		key    string
	}
}

type application struct {
	config  config
	logger  *zap.SugaredLogger
	version string
	DB      models.DBModel
}

func (app *application) serve() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", app.config.port),
		Handler:           app.routes(),
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	app.logger.Info("Starting back-end server in ", app.config.env, " mode on port ", app.config.port)

	return srv.ListenAndServe()
}

func main() {
	loggerInit, _ := zap.NewProduction()
	defer loggerInit.Sync()
	logger := loggerInit.Sugar()

	var cfg config

	flag.IntVar(&cfg.port, "port", 4001, "Server port to listen on")
	flag.StringVar(&cfg.env, "env", "develop", "Application environment {develop|prod|maintenance}")
	flag.Parse()

	cfg.db.dsn = os.Getenv("DSN")
	cfg.stripe.key = os.Getenv("STRIPE_KEY")
	cfg.stripe.secret = os.Getenv("STRIPE_SECRET")

	conn, err := driver.OpenDB(cfg.db.dsn)
	if err != nil {
		logger.Fatal("unable to connect to database ", err)
	}
	defer conn.Close()

	app := &application{
		config:  cfg,
		logger:  logger,
		version: version,
		DB:      models.DBModel{DB: conn},
	}

	if err := app.serve(); err != nil {
		app.logger.Fatal("unable to start the application ", err)
	}

}
