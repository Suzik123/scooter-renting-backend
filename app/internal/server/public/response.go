package public

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/server/middleware"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
)

// WriteJSON writes a standard success envelope to the response.
func WriteJSON(c fiber.Ctx, status int, data any) error {
	reqID := resolveRequestID(c)
	c.Set("X-Request-ID", reqID)
	return c.Status(status).JSON(resp.Envelope{
		Data: data,
		Meta: resp.Meta{
			RequestID: reqID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// WriteCreated is a convenience wrapper for 201 responses.
func WriteCreated(c fiber.Ctx, data any) error {
	return WriteJSON(c, http.StatusCreated, data)
}

// WriteNoContent sets the X-Request-ID header and returns 204.
func WriteNoContent(c fiber.Ctx) error {
	c.Set("X-Request-ID", resolveRequestID(c))
	return c.SendStatus(http.StatusNoContent)
}

// WriteError writes an error envelope corresponding to apiErr.
// apiErr may be nil, in which case a 500 is returned.
func WriteError(c fiber.Ctx, apiErr *resp.APIError) error {
	if apiErr == nil {
		apiErr = resp.ErrInternal()
	}
	reqID := resolveRequestID(c)
	c.Set("X-Request-ID", reqID)
	return c.Status(apiErr.HTTPCode).JSON(resp.ErrorEnvelope{Error: apiErr})
}

// WriteDomain maps a domain error to an APIError envelope.
func WriteDomain(c fiber.Ctx, err error) error {
	return WriteError(c, resp.FromDomain(err))
}

// resolveRequestID pulls the request id from locals, generating a new uuid
// when absent (should not happen once RequestID middleware is registered).
func resolveRequestID(c fiber.Ctx) string {
	if id := middleware.RequestIDFromCtx(c); id != "" {
		return id
	}
	return uuid.NewString()
}
