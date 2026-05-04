package stripeclient

import (
	stripe "github.com/stripe/stripe-go/v82"
)

// Client wraps stripe-go with the configuration needed by the API.
type Client struct {
	sc            *stripe.Client
	webhookSecret string
}

// New constructs a Stripe client from the secret and webhook signing key.
func New(secretKey, webhookSecret string) *Client {
	return &Client{
		sc:            stripe.NewClient(secretKey),
		webhookSecret: webhookSecret,
	}
}

// stringPtr is a small helper for the many *string params Stripe expects.
func stringPtr(s string) *string { return &s }

// boolPtr is a small helper for the many *bool params Stripe expects.
func boolPtr(b bool) *bool { return &b }

// int64Ptr is a small helper for *int64 params Stripe expects.
func int64Ptr(v int64) *int64 { return &v }

// asString returns the string value of a Stripe enum.
func asString[T ~string](v T) string { return string(v) }
