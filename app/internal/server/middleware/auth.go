package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
)

// JWTAuth validates the Authorization bearer token, populates *Identity in
// locals, and rejects unauthenticated requests with a 401 envelope.
func (m *Middleware) JWTAuth(c fiber.Ctx) error {
	h := c.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return writeAPIError(c, resp.ErrUnauthorized("missing bearer token"))
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	if token == "" || m.auth == nil {
		return writeAPIError(c, resp.ErrUnauthorized("invalid token"))
	}
	userID, role, err := m.auth.Parse(token)
	if err != nil {
		return writeAPIError(c, resp.ErrUnauthorized("invalid token"))
	}
	c.Locals(KeyIdentity, &Identity{UserID: userID, Role: role})
	return c.Next()
}

// writeAPIError writes an APIError envelope and stops the middleware chain.
func writeAPIError(c fiber.Ctx, e *resp.APIError) error {
	reqID := RequestIDFromCtx(c)
	c.Set("Content-Type", "application/json")
	if reqID != "" {
		c.Set("X-Request-ID", reqID)
	}
	return c.Status(e.HTTPCode).JSON(resp.ErrorEnvelope{Error: e})
}
