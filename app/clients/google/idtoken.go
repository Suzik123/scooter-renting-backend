package googleclient

import (
	"context"
	"fmt"
)

// VerifyIDToken validates a raw Google id_token against the configured
// audience and returns the trimmed claim view.
func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string) (*Claims, error) {
	payload, err := c.validator.Validate(ctx, rawIDToken, c.audience)
	if err != nil {
		return nil, fmt.Errorf("idtoken.Validate: %w", err)
	}
	out := &Claims{Subject: payload.Subject}
	if v, ok := payload.Claims["email"].(string); ok {
		out.Email = v
	}
	if v, ok := payload.Claims["email_verified"].(bool); ok {
		out.EmailVerified = v
	}
	if v, ok := payload.Claims["given_name"].(string); ok {
		out.GivenName = v
	}
	if v, ok := payload.Claims["family_name"].(string); ok {
		out.FamilyName = v
	}
	if v, ok := payload.Claims["name"].(string); ok {
		out.Name = v
	}
	if v, ok := payload.Claims["picture"].(string); ok {
		out.Picture = v
	}
	if v, ok := payload.Claims["hd"].(string); ok {
		out.HostedDomain = v
	}
	return out, nil
}
