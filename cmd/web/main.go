package main

import (
	"encoding/gob"
	"fmt"
	"go-stripe/internal/driver"
	"go-stripe/internal/models"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"
	"go.uber.org/zap"
)

const version = "1.0.0"

// const cssVersion = "1"

var session *scs.SessionManager

type config struct {
	port int
	env  string
	api  string
	db   struct {
		dsn string
	}
	stripe struct {
		secret string
		key    string
	}
	secretKey string
	frontend  string
}

type application struct {
	config        config
	logger        *zap.SugaredLogger
	templateCache map[string]*template.Template
	version       string
	DB            models.DBModel
	Session       *scs.SessionManager
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

	app.logger.Info("Starting front-end server in ", app.config.env, " mode on port ", app.config.port)

	return srv.ListenAndServe()
}

func main() {
	// register type for session
	gob.Register(TransactionData{})

	// initialize zap sugar logger
	logger := zap.NewExample().Sugar()
	defer func() {
		err := logger.Sync()
		if err != nil {
			log.Fatal("failed to initialize zap logger: ", err)
		}
	}()

	// setup session
	session = scs.New()
	session.Lifetime = 24 * time.Hour

	// setup application config
	var cfg config

	port, err := strconv.Atoi(os.Getenv("FRONTEND_PORT"))
	if err != nil {
		logger.Fatal("unable to get port from env vars: ", err)
	}
	cfg.port = port
	cfg.env = os.Getenv("ENV")

	cfg.api = os.Getenv("BACKEND_URL") + ":" + os.Getenv("BACKEND_PORT")

	cfg.db.dsn = os.Getenv("DSN")
	cfg.stripe.key = os.Getenv("STRIPE_KEY")
	cfg.stripe.secret = os.Getenv("STRIPE_SECRET")

	cfg.secretKey = os.Getenv("SECRET_KEY")
	cfg.frontend = os.Getenv("FRONTEND_URL") + ":" + os.Getenv("FRONTEND_PORT")

	// setup template data
	tc := make(map[string]*template.Template)

	// establish database connection
	conn, err := driver.OpenDB(cfg.db.dsn)
	if err != nil {
		logger.Fatal("unable to connect to database ", err)
	}
	defer conn.Close()

	session.Store = mysqlstore.New(conn)

	// initialize application
	app := &application{
		config:        cfg,
		logger:        logger,
		templateCache: tc,
		version:       version,
		DB:            models.DBModel{DB: conn},
		Session:       session,
	}

	go app.ListenToWsChannel()

	// serve application
	if err := app.serve(); err != nil {
		app.logger.Fatal("unable to start the application ", err)
	}

}
