package main

import "net/http"

func (app *application) CreateAndSend(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("hello world")
}
