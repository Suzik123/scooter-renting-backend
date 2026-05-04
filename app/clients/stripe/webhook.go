package stripeclient

import (
	"fmt"

	"github.com/stripe/stripe-go/v82/webhook"
)

// VerifySignature validates a Stripe-Signature header against the webhook
// signing secret and returns a trimmed-down Event view.
func (c *Client) VerifySignature(payload []byte, sigHeader string) (*Event, error) {
	ev, err := webhook.ConstructEvent(payload, sigHeader, c.webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("stripe.VerifyWebhook: %w", err)
	}
	out := &Event{
		ID:   ev.ID,
		Type: string(ev.Type),
	}
	if ev.Data != nil {
		out.Data = ev.Data.Raw
	}
	return out, nil
}
