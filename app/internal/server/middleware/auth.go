package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
)

// JWTAuth validates the Authorization bearer token, populates *Identity in
// locals, checks the JTI blacklist, and rejects unauthenticated requests
// with a 401 envelope.
//
// FAIL-OPEN: see PLAN section 5 — when the blacklist backend (Redis) is
// unreachable, the middleware logs and continues with the request rather
// than locking every authenticated user out. The trade-off is that a
// logout-then-blacklist-outage window can leave a revoked token usable
// until Redis recovers; we accept this for v1.
func (m *Middleware) JWTAuth(c fiber.Ctx) error {
	h := c.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return writeAPIError(c, resp.ErrUnauthorized("missing bearer token"))
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	if token == "" || m.auth == nil {
		return writeAPIError(c, resp.ErrUnauthorized("invalid token"))
	}
	userID, role, jti, err := m.auth.Parse(token)
	if err != nil {
		return writeAPIError(c, resp.ErrUnauthorized("invalid token"))
	}

	if jti != "" {
		revoked, rerr := m.auth.IsRevoked(c.Context(), jti)
		if rerr != nil {
			// FAIL-OPEN: see PLAN section 5
			m.log.Warn("auth.JWTAuth: blacklist check failed; failing open",
				zap.String("jti", jti),
				zap.Error(rerr),
			)
		} else if revoked {
			return writeAPIError(c, resp.ErrUnauthorized("token revoked"))
		}
	}

	c.Locals(KeyIdentity, &Identity{UserID: userID, Role: role, JTI: jti, Token: token})
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
