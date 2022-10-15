package main

import (
	"flag"
	"fmt"
	"go-stripe/internal/driver"
	"go-stripe/internal/models"
	"log"
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

// serve application
func (app *application) serve() error {
	// initialize http server
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
	// initialize zap sugar logger
	logger := zap.NewExample().Sugar()
	defer func() {
		err := logger.Sync()
		if err != nil {
			log.Fatal("failed to initialize zap logger: ", err)
		}
	}()

	// setup application config
	var cfg config

	flag.IntVar(&cfg.port, "port", 4001, "Server port to listen on")
	flag.StringVar(&cfg.env, "env", "develop", "Application environment {develop|prod|maintenance}")
	flag.Parse()

	cfg.db.dsn = os.Getenv("DSN")
	cfg.stripe.key = os.Getenv("STRIPE_KEY")
	cfg.stripe.secret = os.Getenv("STRIPE_SECRET")

	// establish database connection
	conn, err := driver.OpenDB(cfg.db.dsn)
	if err != nil {
		logger.Fatal("unable to connect to database ", err)
	}
	defer conn.Close()

	// initialize application
	app := &application{
		config:  cfg,
		logger:  logger,
		version: version,
		DB:      models.DBModel{DB: conn},
	}

	// serve application
	if err := app.serve(); err != nil {
		app.logger.Fatal("unable to start the application ", err)
	}

}
