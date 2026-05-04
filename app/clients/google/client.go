package googleclient

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// Client wraps a google idtoken.Validator with the configured audience.
type Client struct {
	validator *idtoken.Validator
	audience  string
}

// New constructs a Client by initializing an idtoken validator. The audience
// must equal the OAuth client_id used by the frontend.
func New(ctx context.Context, clientID string) (*Client, error) {
	v, err := idtoken.NewValidator(ctx)
	if err != nil {
		return nil, fmt.Errorf("idtoken.NewValidator: %w", err)
	}
	return &Client{validator: v, audience: clientID}, nil
}
