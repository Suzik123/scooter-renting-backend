// Package events defines the typed envelopes published from the api process
// and consumed from the worker process. Concrete transport (RabbitMQ) lives
// in app/pkg/messaging — this package is transport-agnostic so service-layer
// code does not depend on AMQP.
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Event type constants. Match the routing keys defined in app/pkg/messaging.
const (
	TypePaymentSucceeded       = "payment.succeeded"
	TypePaymentFailed          = "payment.failed"
	TypeOfflinePaymentApproved = "payment.offline_approved"
	TypeRentalStarted          = "rental.started"
	TypeRentalCompleted        = "rental.completed"
)

const (
	sourceAPI = "uniscoot-api"
	version   = 1
)

// Publisher is the narrow interface the service layer uses to publish events.
// Concrete impl lives in app/pkg/messaging.Publisher.
type Publisher interface {
	Publish(ctx context.Context, evt Envelope) error
}

// NopPublisher is a Publisher that discards every event. Used in tests or
// when messaging is not wired (e.g. CLI tools that share fx providers).
type NopPublisher struct{}

// Publish satisfies Publisher and never returns an error.
func (NopPublisher) Publish(_ context.Context, _ Envelope) error { return nil }

// Envelope wraps every published event with stable metadata.
type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Source     string          `json:"source"`
	Version    int             `json:"version"`
	Payload    json.RawMessage `json:"payload"`
}

// NewEnvelope marshals payload into the envelope's Payload field.
func NewEnvelope(typ string, payload any) (Envelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:         uuid.NewString(),
		Type:       typ,
		OccurredAt: time.Now().UTC(),
		Source:     sourceAPI,
		Version:    version,
		Payload:    body,
	}, nil
}

// ---- Payload types

// PaymentSucceeded is published after a payment row flips to 'succeeded'.
type PaymentSucceeded struct {
	PaymentID  uuid.UUID        `json:"payment_id"`
	UserID     uuid.UUID        `json:"user_id"`
	RentalID   *uuid.UUID       `json:"rental_id,omitempty"`
	Amount     decimal.Decimal  `json:"amount"`
	Currency   string           `json:"currency"`
	UserEmail  string           `json:"user_email"`
	UserName   string           `json:"user_name"`
	OccurredAt time.Time        `json:"occurred_at"`
}

// PaymentFailed is published after a payment row flips to 'failed'.
type PaymentFailed struct {
	PaymentID     uuid.UUID       `json:"payment_id"`
	UserID        uuid.UUID       `json:"user_id"`
	RentalID      *uuid.UUID      `json:"rental_id,omitempty"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	FailureReason string          `json:"failure_reason,omitempty"`
	UserEmail     string          `json:"user_email"`
	UserName      string          `json:"user_name"`
	OccurredAt    time.Time       `json:"occurred_at"`
}

// OfflinePaymentApproved is published after an admin records a manual offline
// payment for a completed rental.
type OfflinePaymentApproved struct {
	PaymentID    uuid.UUID       `json:"payment_id"`
	UserID       uuid.UUID       `json:"user_id"`
	RentalID     uuid.UUID       `json:"rental_id"`
	Amount       decimal.Decimal `json:"amount"`
	Currency     string          `json:"currency"`
	ApprovedBy   uuid.UUID       `json:"approved_by"`
	Note         string          `json:"note,omitempty"`
	UserEmail    string          `json:"user_email"`
	UserName     string          `json:"user_name"`
	OccurredAt   time.Time       `json:"occurred_at"`
}

// RentalStarted is published after a rental is opened.
type RentalStarted struct {
	RentalID  uuid.UUID `json:"rental_id"`
	UserID    uuid.UUID `json:"user_id"`
	ScooterID uuid.UUID `json:"scooter_id"`
	StartedAt time.Time `json:"started_at"`
}

// RentalCompleted is published after a rental is closed (regardless of the
// payment outcome).
type RentalCompleted struct {
	RentalID   uuid.UUID       `json:"rental_id"`
	UserID     uuid.UUID       `json:"user_id"`
	ScooterID  uuid.UUID       `json:"scooter_id"`
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    time.Time       `json:"ended_at"`
	DurationS  int64           `json:"duration_s"`
	DistanceM  int             `json:"distance_m"`
	TotalCost  decimal.Decimal `json:"total_cost"`
	Currency   string          `json:"currency"`
	UserEmail  string          `json:"user_email"`
	UserName   string          `json:"user_name"`
}
