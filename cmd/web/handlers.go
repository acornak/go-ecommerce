package main

import "net/http"

func (app *application) virtualTerminal(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "terminal", nil); err != nil {
		app.logger.Fatal(err)
	}
}
