package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// headerRequestID is the HTTP header used to propagate request ids.
const headerRequestID = "X-Request-ID"

// RequestID reads (or generates) a request id and stores it both in locals
// and in the response header.
func (m *Middleware) RequestID(c fiber.Ctx) error {
	id := c.Get(headerRequestID)
	if id == "" {
		id = uuid.NewString()
	}
	c.Set(headerRequestID, id)
	c.Locals(KeyRequestID, id)
	return c.Next()
}

// RequestIDFromCtx returns the request id stored in fiber.Ctx locals or "".
func RequestIDFromCtx(c fiber.Ctx) string {
	if v, ok := c.Locals(KeyRequestID).(string); ok {
		return v
	}
	return ""
}
