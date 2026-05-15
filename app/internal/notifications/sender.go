// Package notifications consumes events from RabbitMQ and renders + sends
// templated emails via SMTP (MailHog in development).
package notifications

import (
	"context"
	"fmt"

	"gopkg.in/gomail.v2"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
)

// Sender is the narrow interface used by the consumers. The default impl is
// SMTPSender; tests can substitute a fake.
type Sender interface {
	SendEmail(ctx context.Context, to, subject, html, text string) error
}

// SMTPSender talks to an SMTP server using gomail.v2. Compatible with
// MailHog (no auth, plain port) and with real SMTP providers when User /
// Password are set.
type SMTPSender struct {
	host     string
	port     int
	from     string
	user     string
	password string
}

// NewSender constructs a SMTPSender configured from cfg.SMTP.
func NewSender(cfg *config.Config) Sender {
	return &SMTPSender{
		host:     cfg.SMTP.Host,
		port:     cfg.SMTP.Port,
		from:     cfg.SMTP.From,
		user:     cfg.SMTP.User,
		password: cfg.SMTP.Password,
	}
}

// SendEmail composes a multi-part message and dials the SMTP server.
// MailHog accepts unauthenticated plain connections so an empty user causes
// gomail to skip the AUTH step.
func (s *SMTPSender) SendEmail(_ context.Context, to, subject, html, text string) error {
	if s == nil || s.host == "" {
		return fmt.Errorf("smtp sender not configured")
	}
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	if text != "" {
		m.SetBody("text/plain", text)
		if html != "" {
			m.AddAlternative("text/html", html)
		}
	} else if html != "" {
		m.SetBody("text/html", html)
	}

	d := gomail.NewDialer(s.host, s.port, s.user, s.password)
	// MailHog: skip TLS / auth. Real providers will set User/Password.
	if s.user == "" {
		d.SSL = false
	}
	return d.DialAndSend(m)
}
