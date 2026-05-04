package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	PaymentPending   = "pending"
	PaymentSucceeded = "succeeded"
	PaymentFailed    = "failed"
	PaymentRefunded  = "refunded"
)

const (
	PaymentMethodCard       = "card"
	PaymentMethodApplePay   = "apple_pay"
	PaymentMethodGooglePay  = "google_pay"
)

type Payment struct {
	ID                uuid.UUID       `db:"payment_id" json:"payment_id"`
	UserID            uuid.UUID       `db:"user_id" json:"user_id"`
	RentalID          *uuid.UUID      `db:"rental_id" json:"rental_id,omitempty"`
	Amount            decimal.Decimal `db:"amount" json:"amount"`
	Currency          string          `db:"currency" json:"currency"`
	PaymentMethod     string          `db:"payment_method" json:"payment_method"`
	Status            string          `db:"status" json:"status"`
	ProviderPaymentID *string         `db:"provider_payment_id" json:"provider_payment_id,omitempty"`
	FailureReason     *string         `db:"failure_reason" json:"failure_reason,omitempty"`
	TransactionDate   time.Time       `db:"transaction_date" json:"transaction_date"`
	UpdatedAt         time.Time       `db:"updated_at" json:"updated_at"`
}
