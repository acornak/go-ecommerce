package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/phpdave11/gofpdf"
	"github.com/phpdave11/gofpdf/contrib/gofpdi"
	"go.uber.org/zap"
)

// type for all orders
type Order struct {
	ID        int       `json:"id"`
	Quantity  int       `json:"quantity"`
	Amount    int       `json:"amount"`
	Product   string    `json:"product"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (app *application) CreateAndSend(w http.ResponseWriter, r *http.Request) {
	var order Order

	err := app.readJSON(w, r, &order)
	if err != nil {
		app.logger.Error("error reading json: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	err = app.createInvoicePDF(order)
	if err != nil {
		app.logger.Error("error creating invoice: ", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	attachments := []string{
		fmt.Sprintf("./invoices/%d.pdf", order.ID),
	}

	err = app.SendMail("info@widgets.com", order.Email, "Your Invoice", "invoice", attachments, nil)
	if err != nil {
		app.logger.Error("error sending email", err)
		if err = app.badRequest(w, r, err); err != nil {
			app.logger.Error(err)
		}
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	resp.Error = false
	resp.Message = fmt.Sprintf("Invoice %d.pdf created and sent to %s", order.ID, order.Email)

	app.logger.Info(resp.Message)

	if err = app.writeJson(w, http.StatusOK, resp); err != nil {
		app.logger.Error("error writing response: ", zap.Error(err))
	}
}

func (app *application) createInvoicePDF(order Order) error {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(10, 13, 10)
	pdf.SetAutoPageBreak(true, 0)

	importer := gofpdi.NewImporter()

	t := importer.ImportPage(pdf, "./pdf-templates/invoice.pdf", 1, "/MediaBox")

	pdf.AddPage()
	importer.UseImportedTemplate(pdf, t, 0, 0, 215.9, 0)

	// invoice head
	pdf.SetX(10)
	pdf.SetY(50)
	pdf.SetFont("Times", "", 10)
	pdf.CellFormat(97, 8, fmt.Sprintf("Attention: %s %s", order.FirstName, order.LastName), "", 0, "L", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(97, 8, order.Email, "", 0, "L", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(97, 8, order.CreatedAt.Format("2006-01-02"), "", 0, "L", false, 0, "")

	// invoice items
	pdf.SetX(58)
	pdf.SetY(93)
	pdf.CellFormat(155, 8, order.Product, "", 0, "L", false, 0, "")

	pdf.SetX(166)
	pdf.CellFormat(20, 8, fmt.Sprint(order.Quantity), "", 0, "C", false, 0, "")

	pdf.SetX(185)
	pdf.CellFormat(20, 8, fmt.Sprintf("%.2f EUR", float32(order.Amount/100.0)), "", 0, "R", false, 0, "")

	invoicePath := fmt.Sprintf("./invoices/%d.pdf", order.ID)

	err := pdf.OutputFileAndClose(invoicePath)
	if err != nil {
		return err
	}

	return nil
}
