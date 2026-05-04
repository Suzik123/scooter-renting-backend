package stripeclient

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v82"
)

// CreateSetupIntent creates an off-session SetupIntent for the customer and
// returns its client_secret and id.
func (c *Client) CreateSetupIntent(ctx context.Context, customerID string) (string, string, error) {
	params := &stripe.SetupIntentCreateParams{
		Customer:           stringPtr(customerID),
		PaymentMethodTypes: []*string{stringPtr("card")},
		Usage:              stringPtr("off_session"),
	}
	si, err := c.sc.V1SetupIntents.Create(ctx, params)
	if err != nil {
		return "", "", fmt.Errorf("stripe.CreateSetupIntent: %w", err)
	}
	return si.ClientSecret, si.ID, nil
}
