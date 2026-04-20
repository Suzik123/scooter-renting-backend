package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
)

// ContentTypeJSON enforces application/json for bodied POST/PUT/PATCH requests.
func (m *Middleware) ContentTypeJSON(c fiber.Ctx) error {
	method := c.Method()
	if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch {
		return c.Next()
	}
	if len(c.Body()) == 0 {
		return c.Next()
	}
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct == "" || !strings.HasPrefix(ct, "application/json") {
		return writeAPIError(c, resp.ErrBadRequest(resp.CodeBadRequest, "content-type must be application/json"))
	}
	return c.Next()
}
