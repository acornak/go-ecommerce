package main

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
	"time"

	mail "github.com/xhit/go-simple-mail/v2"
)

//go:embed templates
var emailTemplateFS embed.FS

func (app *application) SendMail(from, to, subject, tmpl string, attachments []string, data any) error {
	formattedMessage, err := app.renderTemplate(tmpl, "html", data)
	if err != nil {
		return err
	}

	plainMessage, err := app.renderTemplate(tmpl, "plain", data)
	if err != nil {
		return err
	}

	server := mail.NewSMTPClient()
	server.Host = app.config.smtp.host
	server.Port = app.config.smtp.port
	server.Username = app.config.smtp.username
	server.Password = app.config.smtp.password
	server.Encryption = mail.EncryptionTLS
	server.KeepAlive = false
	server.ConnectTimeout = 30 * time.Second
	server.SendTimeout = 30 * time.Second

	smtpClient, err := server.Connect()
	if err != nil {
		return err
	}

	email := mail.NewMSG()
	email.SetFrom(from).
		AddTo(to).
		SetSubject(subject).
		SetBody(mail.TextHTML, formattedMessage).
		AddAlternative(mail.TextPlain, plainMessage)

	if len(attachments) > 0 {
		for _, v := range attachments {
			email.AddAttachment(v)
		}
	}

	if err = email.Send(smtpClient); err != nil {
		return err
	}

	return nil
}

func (app *application) renderTemplate(tmpl, mailType string, data any) (string, error) {
	templateToRender := fmt.Sprintf("templates/%s.%s.gohtml", tmpl, mailType)
	t, err := template.New("email-"+mailType).ParseFS(emailTemplateFS, templateToRender)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return "", err
	}

	return tpl.String(), nil
}
