package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
)

// AdminOnly allows requests only from identities with the admin role.
func (m *Middleware) AdminOnly(c fiber.Ctx) error {
	ident, _ := c.Locals(KeyIdentity).(*Identity)
	if ident == nil || ident.Role != models.RoleAdmin {
		return writeAPIError(c, resp.ErrForbidden("admin only"))
	}
	return c.Next()
}
