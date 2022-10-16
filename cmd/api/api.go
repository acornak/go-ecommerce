package main

import (
	"fmt"
	"go-stripe/internal/driver"
	"go-stripe/internal/models"
	"log"
	"net/http"
	"os"
	"strconv"
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
	smtp struct {
		host     string
		port     int
		username string
		password string
	}
	secretKey string
	frontend  string
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

	port, err := strconv.Atoi(os.Getenv("BACKEND_PORT"))
	if err != nil {
		logger.Fatal("unable to get port from env vars: ", err)
	}
	cfg.port = port
	cfg.env = os.Getenv("ENV")

	cfg.db.dsn = os.Getenv("DSN")
	cfg.stripe.key = os.Getenv("STRIPE_KEY")
	cfg.stripe.secret = os.Getenv("STRIPE_SECRET")

	cfg.smtp.host = os.Getenv("SMTP_HOST")
	smtpPort, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		logger.Fatal("unable to get smtp port from env vars: ", err)
	}
	cfg.smtp.port = smtpPort
	cfg.smtp.username = os.Getenv("SMTP_USERNAME")
	cfg.smtp.password = os.Getenv("SMTP_PASSWORD")

	cfg.secretKey = os.Getenv("SECRET_KEY")
	cfg.frontend = os.Getenv("FRONTEND_URL") + ":" + os.Getenv("FRONTEND_PORT")

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
		logger.Fatal("unable to start the application ", err)
	}
}
