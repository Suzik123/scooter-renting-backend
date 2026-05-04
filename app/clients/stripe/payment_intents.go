package stripeclient

import (
	"context"
	"errors"
	"fmt"

	stripe "github.com/stripe/stripe-go/v82"
)

// CreateAndConfirmPaymentIntent issues a confirmed off-session PaymentIntent.
// Stripe API errors representing card declines or similar payment-method
// failures are surfaced as ChargeResult with a non-empty FailureReason and
// Status, instead of returning err. Other errors propagate as err.
func (c *Client) CreateAndConfirmPaymentIntent(ctx context.Context, p ChargeParams) (*ChargeResult, error) {
	params := &stripe.PaymentIntentCreateParams{
		Amount:        int64Ptr(p.AmountMinor),
		Currency:      stringPtr(p.Currency),
		Customer:      stringPtr(p.CustomerID),
		PaymentMethod: stringPtr(p.PaymentMethodID),
		Confirm:       boolPtr(true),
		OffSession:    boolPtr(true),
	}
	if len(p.Metadata) > 0 {
		params.Metadata = p.Metadata
	}
	if p.IdempotencyKey != "" {
		params.IdempotencyKey = stringPtr(p.IdempotencyKey)
	}

	pi, err := c.sc.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		var sErr *stripe.Error
		if errors.As(err, &sErr) && sErr.PaymentIntent != nil {
			return &ChargeResult{
				IntentID:      sErr.PaymentIntent.ID,
				Status:        asString(sErr.PaymentIntent.Status),
				ClientSecret:  sErr.PaymentIntent.ClientSecret,
				FailureReason: sErr.Msg,
			}, nil
		}
		return nil, fmt.Errorf("stripe.CreateAndConfirmPaymentIntent: %w", err)
	}
	return &ChargeResult{
		IntentID:     pi.ID,
		Status:       asString(pi.Status),
		ClientSecret: pi.ClientSecret,
	}, nil
}
