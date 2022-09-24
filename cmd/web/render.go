package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type templateData struct {
	StringMap            map[string]string
	IntMap               map[string]int
	FloatMap             map[string]float32
	Data                 map[string]any
	CSRFToken            string
	Flash                string
	Warning              string
	Error                string
	IsAuthenticated      int
	API                  string
	CSSVersion           string
	StripeSecretKey      string
	StripePublishableKey string
}

var functions = template.FuncMap{
	"formatCurrency": formatCurrency,
}

// format currency to user friendly format
func formatCurrency(n int) string {
	f := float32(n) / float32(100)
	return fmt.Sprintf("%.2f €", f)
}

//go:embed templates
var templateFS embed.FS

// handle default template data
func (app *application) addDefaultData(td *templateData, r *http.Request) *templateData {
	td.API = app.config.api
	td.StripePublishableKey = app.config.stripe.key
	td.StripeSecretKey = app.config.stripe.secret

	return td
}

// render template
func (app *application) renderTemplate(w http.ResponseWriter, r *http.Request, page string, td *templateData, partials ...string) error {
	var t *template.Template
	var err error

	templateToRender := fmt.Sprintf("templates/%s.page.gohtml", page)
	_, templateInMap := app.templateCache[templateToRender]

	// handle caching pages
	if app.config.env == "prod" && templateInMap {
		t = app.templateCache[templateToRender]
	} else {
		t, err = app.parseTemplate(partials, page, templateToRender)
		if err != nil {
			app.logger.Error("failed to parse template: ", zap.Error(err))
			return err
		}
	}

	if td == nil {
		td = &templateData{}
	}
	td = app.addDefaultData(td, r)

	err = t.Execute(w, td)
	if err != nil {
		app.logger.Error("failed to execute template: ", zap.Error(err))
		return err
	}

	return nil
}

// parse gohtml templates
func (app *application) parseTemplate(partials []string, page, templateToRender string) (*template.Template, error) {
	var t *template.Template
	var err error

	// build partials
	if len(partials) > 0 {
		for i, x := range partials {
			partials[i] = fmt.Sprintf("templates/%s.partial.gohtml", x)
		}
		t, err = template.New(fmt.Sprintf("%s.page.gohtml", page)).Funcs(functions).ParseFS(templateFS, "templates/base.layout.gohtml", strings.Join(partials, ","), templateToRender)
	} else {
		t, err = template.New(fmt.Sprintf("%s.page.gohtml", page)).Funcs(functions).ParseFS(templateFS, "templates/base.layout.gohtml", templateToRender)
	}

	if err != nil {
		app.logger.Error("failed to create template: ", zap.Error(err))
		return nil, err
	}

	app.templateCache[templateToRender] = t

	return t, nil
}
