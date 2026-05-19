package notifications

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	texttemplate "text/template"
)

//go:embed templates/*.html templates/*.txt
var templatesFS embed.FS

// templateConfig describes the subject + filename pair for an event type.
type templateConfig struct {
	subject  string
	htmlFile string
	textFile string
}

// templatesByEvent maps event types to their email templates.
var templatesByEvent = map[string]templateConfig{
	"payment.succeeded": {
		subject:  "UniScoot — payment received",
		htmlFile: "templates/payment_succeeded.html",
		textFile: "templates/payment_succeeded.txt",
	},
	"payment.failed": {
		subject:  "UniScoot — payment failed",
		htmlFile: "templates/payment_failed.html",
		textFile: "templates/payment_failed.txt",
	},
	"payment.offline_approved": {
		subject:  "UniScoot — offline payment recorded",
		htmlFile: "templates/offline_payment_approved.html",
		textFile: "templates/offline_payment_approved.txt",
	},
	"rental.completed": {
		subject:  "UniScoot — ride summary",
		htmlFile: "templates/rental_completed.html",
		textFile: "templates/rental_completed.txt",
	},
	"password_reset.requested": {
		subject:  "UniScoot — password reset",
		htmlFile: "templates/password_reset_requested.html",
		textFile: "templates/password_reset_requested.txt",
	},
}

// Render produces (subject, html, text) for the named template using data.
// Returns an empty subject when the event type has no associated template
// (signals the consumer to drop the message).
func Render(eventType string, data any) (subject, html, text string, err error) {
	tc, ok := templatesByEvent[eventType]
	if !ok {
		return "", "", "", nil
	}

	htmlBytes, err := templatesFS.ReadFile(tc.htmlFile)
	if err != nil {
		return "", "", "", fmt.Errorf("read html template: %w", err)
	}
	textBytes, err := templatesFS.ReadFile(tc.textFile)
	if err != nil {
		return "", "", "", fmt.Errorf("read text template: %w", err)
	}

	htmlTpl, err := template.New(tc.htmlFile).Parse(string(htmlBytes))
	if err != nil {
		return "", "", "", fmt.Errorf("parse html template: %w", err)
	}
	textTpl, err := texttemplate.New(tc.textFile).Parse(string(textBytes))
	if err != nil {
		return "", "", "", fmt.Errorf("parse text template: %w", err)
	}

	var hbuf bytes.Buffer
	if err := htmlTpl.Execute(&hbuf, data); err != nil {
		return "", "", "", fmt.Errorf("exec html template: %w", err)
	}
	var tbuf bytes.Buffer
	if err := textTpl.Execute(&tbuf, data); err != nil {
		return "", "", "", fmt.Errorf("exec text template: %w", err)
	}

	return tc.subject, hbuf.String(), strings.TrimSpace(tbuf.String()), nil
}
