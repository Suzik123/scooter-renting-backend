package stripeclient

import "encoding/json"

// PaymentMethodView is the trimmed-down view of a Stripe PaymentMethod
// returned to the application service layer.
type PaymentMethodView struct {
	ID        string `json:"id"`
	Brand     string `json:"brand"`
	Last4     string `json:"last4"`
	ExpMonth  int    `json:"exp_month"`
	ExpYear   int    `json:"exp_year"`
	IsDefault bool   `json:"is_default"`
}

// ChargeParams captures inputs for an off-session charge.
type ChargeParams struct {
	CustomerID      string
	PaymentMethodID string
	AmountMinor     int64
	Currency        string
	IdempotencyKey  string
	Metadata        map[string]string
}

// ChargeResult captures outputs of an off-session charge.
type ChargeResult struct {
	IntentID      string
	Status        string
	ClientSecret  string
	FailureReason string
}

// Event is the trimmed-down view of a verified Stripe webhook event.
type Event struct {
	ID   string
	Type string
	Data json.RawMessage
}
