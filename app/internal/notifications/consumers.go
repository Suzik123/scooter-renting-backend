package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/messaging"
)

// RegisterConsumers wires the email-send queues that consume from
// payments.events and notifications.events. Best-effort: registration errors
// are returned and surface from fx Invoke.
func RegisterConsumers(mq *messaging.Client, sender Sender, log *zap.Logger) error {
	if mq == nil {
		return errors.New("messaging client is nil")
	}
	if log == nil {
		log = zap.NewNop()
	}
	if sender == nil {
		return errors.New("sender is nil")
	}

	// Single durable queue that fans in from both exchanges. The handler
	// inspects the envelope.Type to render the right template.
	const queue = "notifications.email.send"

	handler := func(_ context.Context, body []byte, headers amqp.Table) error {
		_ = headers
		var env events.Envelope
		if err := json.Unmarshal(body, &env); err != nil {
			log.Error("notifications: bad envelope", zap.Error(err))
			// Drop poison message — returning an error would requeue it.
			return nil
		}
		return dispatch(env, sender, log)
	}

	// Bind to payments.events for every payment + rental routing key.
	for _, key := range []string{
		"payment.succeeded",
		"payment.failed",
		"payment.offline_approved",
		"rental.completed",
	} {
		if err := mq.Consume(queue, messaging.ExchangePayments, key, handler); err != nil {
			return fmt.Errorf("consume %s/%s: %w", messaging.ExchangePayments, key, err)
		}
	}

	// Bind to notifications.events for explicit email-send fan-out (future).
	if err := mq.Consume(queue, messaging.ExchangeNotifications, "notification.email.#", handler); err != nil {
		return fmt.Errorf("consume notifications: %w", err)
	}

	log.Info("notifications consumers registered", zap.String("queue", queue))
	return nil
}

// dispatch renders the email for env and hands it to the SMTP sender. Unknown
// event types are dropped silently. Returns a non-nil error to signal nack.
func dispatch(env events.Envelope, sender Sender, log *zap.Logger) error {
	to, err := recipientFor(env)
	if err != nil {
		log.Warn("notifications: no recipient", zap.String("type", env.Type), zap.Error(err))
		return nil
	}
	if to == "" {
		log.Warn("notifications: empty recipient", zap.String("type", env.Type), zap.String("event_id", env.ID))
		return nil
	}

	subject, html, text, err := Render(env.Type, payloadFor(env))
	if err != nil {
		log.Error("notifications: render", zap.String("type", env.Type), zap.Error(err))
		return nil
	}
	if subject == "" {
		// Unknown type — silently drop.
		return nil
	}

	if err := sender.SendEmail(context.Background(), to, subject, html, text); err != nil {
		log.Error("notifications: send",
			zap.String("type", env.Type),
			zap.String("event_id", env.ID),
			zap.String("to", to),
			zap.Error(err),
		)
		// Requeue for retry on SMTP transient errors.
		return err
	}
	log.Info("notifications: email sent",
		zap.String("type", env.Type),
		zap.String("event_id", env.ID),
		zap.String("to", to),
	)
	return nil
}

// recipientFor extracts the recipient email from the envelope payload. Only
// types that carry a user_email field have a non-empty recipient.
func recipientFor(env events.Envelope) (string, error) {
	switch env.Type {
	case events.TypePaymentSucceeded:
		var p events.PaymentSucceeded
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", err
		}
		return p.UserEmail, nil
	case events.TypePaymentFailed:
		var p events.PaymentFailed
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", err
		}
		return p.UserEmail, nil
	case events.TypeOfflinePaymentApproved:
		var p events.OfflinePaymentApproved
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", err
		}
		return p.UserEmail, nil
	case events.TypeRentalCompleted:
		var p events.RentalCompleted
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", err
		}
		return p.UserEmail, nil
	case events.TypePasswordResetRequested:
		var p events.PasswordResetRequested
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", err
		}
		return p.UserEmail, nil
	}
	return "", nil
}

// payloadFor unmarshals env.Payload into the concrete struct matching its Type.
func payloadFor(env events.Envelope) any {
	switch env.Type {
	case events.TypePaymentSucceeded:
		var p events.PaymentSucceeded
		_ = json.Unmarshal(env.Payload, &p)
		return p
	case events.TypePaymentFailed:
		var p events.PaymentFailed
		_ = json.Unmarshal(env.Payload, &p)
		return p
	case events.TypeOfflinePaymentApproved:
		var p events.OfflinePaymentApproved
		_ = json.Unmarshal(env.Payload, &p)
		return p
	case events.TypeRentalCompleted:
		var p events.RentalCompleted
		_ = json.Unmarshal(env.Payload, &p)
		return p
	case events.TypePasswordResetRequested:
		var p events.PasswordResetRequested
		_ = json.Unmarshal(env.Payload, &p)
		return p
	}
	return nil
}
