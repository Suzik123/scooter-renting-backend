package messaging

import (
	"context"

	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
)

// Publisher is a typed wrapper around Client that knows how to route an
// events.Envelope to the correct exchange + routing key.
//
// It also implements the events.Publisher interface (single Publish method),
// breaking the dependency cycle that would otherwise exist between the
// messaging and events packages.
type Publisher struct {
	c *Client
}

// NewPublisher constructs a Publisher and ensures the topic exchanges that
// events route to are declared.
func NewPublisher(c *Client) (*Publisher, error) {
	if err := c.DeclareExchange(ExchangePayments, "topic"); err != nil {
		return nil, err
	}
	if err := c.DeclareExchange(ExchangeNotifications, "topic"); err != nil {
		return nil, err
	}
	return &Publisher{c: c}, nil
}

// Publish maps the envelope's Type to the (exchange, routing_key) tuple
// declared in events/routing.go and forwards to the AMQP client.
func (p *Publisher) Publish(ctx context.Context, evt events.Envelope) error {
	exchange, key := events.RouteFor(evt.Type)
	if exchange == "" || key == "" {
		// Unknown event type — drop silently rather than fail callers.
		return nil
	}
	return p.c.PublishJSON(ctx, exchange, key, evt, nil)
}
