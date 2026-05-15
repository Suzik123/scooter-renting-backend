package stripeclient

import (
	"fmt"

	"github.com/stripe/stripe-go/v82/webhook"
)

// VerifySignature validates a Stripe-Signature header against the webhook
// signing secret and returns a trimmed-down Event view.
//
// IgnoreAPIVersionMismatch: stripe-cli's `listen` forwards events stamped
// with the account's Stripe Dashboard API version, which can lag behind the
// pinned stripe-go SDK version. Without this option every dev-forwarded
// event is rejected with an API-version error even when the signature is
// valid. We only read stable fields (id, type, data.object.id,
// last_payment_error.message), so loose deserialization is safe here.
func (c *Client) VerifySignature(payload []byte, sigHeader string) (*Event, error) {
	ev, err := webhook.ConstructEventWithOptions(payload, sigHeader, c.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
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
