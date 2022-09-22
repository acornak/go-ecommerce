package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

const version = "1.0.0"
const cssVersion = "1"

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
}

type application struct {
	config        config
	logger        *zap.SugaredLogger
	templateCache map[string]*template.Template
	version       string
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

	cfg.stripe.key = os.Getenv("STRIPE_KEY")
	cfg.stripe.secret = os.Getenv("STRIPE_SECRET")

	tc := make(map[string]*template.Template)

	app := &application{
		config:        cfg,
		logger:        logger,
		templateCache: tc,
		version:       version,
	}

	if err := app.serve(); err != nil {
		app.logger.Fatal("unable to start the application ", err)
	}

}
