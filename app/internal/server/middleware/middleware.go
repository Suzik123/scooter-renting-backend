package middleware

import (
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
)

// Identity represents the authenticated caller extracted by auth middleware.
type Identity struct {
	UserID uuid.UUID
	Role   string
}

// ContextKey is a typed key used to store values in fiber.Ctx locals.
type ContextKey string

const (
	// KeyRequestID is the locals key under which the request id is stored.
	KeyRequestID ContextKey = "request_id"
	// KeyIdentity is the locals key under which *Identity is stored.
	KeyIdentity ContextKey = "identity"
)

// AuthParser extracts user id and role from a bearer token.
type AuthParser interface {
	Parse(token string) (userID uuid.UUID, role string, err error)
}

// Middleware groups all HTTP middlewares for the API.
type Middleware struct {
	auth AuthParser
	cfg  *config.Config
	log  *zap.Logger
}

// New constructs a Middleware with the provided dependencies.
func New(auth AuthParser, cfg *config.Config, log *zap.Logger) *Middleware {
	if log == nil {
		log = zap.NewNop()
	}
	return &Middleware{auth: auth, cfg: cfg, log: log}
}

// Logger returns the underlying zap logger (used by handlers to log domain events).
func (m *Middleware) Logger() *zap.Logger {
	return m.log
}
