package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/hasher"
)

// UserRepo is the subset of the users repository the auth service depends on.
type UserRepo interface {
	Create(ctx context.Context, u *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type Service struct {
	users     UserRepo
	cfg       *config.Config
	clock     func() time.Time
	blacklist Blacklist

	// Password-reset deps, wired post-construction via SetPasswordResetDeps
	// so the existing constructor + tests keep their current arity.
	prTokens          PasswordResetTokenRepo
	prUsers           PasswordResetUserRepo
	prPub             events.Publisher
	prLog             *zap.Logger
	prTTL             time.Duration
	prFrontendBaseURL string
}

func New(cfg *config.Config, users UserRepo) *Service {
	return &Service{
		users:     users,
		cfg:       cfg,
		clock:     time.Now,
		blacklist: nopBlacklist{},
	}
}

// SetBlacklist injects a JTI blacklist after construction. Used by the fx
// wiring so the auth service stays usable in tests without a Redis dep.
func (s *Service) SetBlacklist(b Blacklist) {
	if b == nil {
		s.blacklist = nopBlacklist{}
		return
	}
	s.blacklist = b
}

// Register creates a new user with role "user" and returns the user with a freshly issued JWT.
func (s *Service) Register(ctx context.Context, email, firstName, lastName, password, phoneNumber string) (*models.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	if email == "" || firstName == "" || password == "" {
		return nil, "", apperrors.Invalid("email, first_name and password are required")
	}

	hash, err := hasher.Hash(password, s.cfg.Bcrypt.Cost)
	if err != nil {
		return nil, "", apperrors.Internal("hash password")
	}

	u := &models.User{
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		PasswordHash: &hash,
		Role:         models.RoleUser,
		Status:       models.UserActive,
	}
	if phoneNumber != "" {
		u.PhoneNumber = &phoneNumber
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, "", err
	}

	token, err := s.IssueJWT(u)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// Login authenticates a user by email and password and returns a freshly issued JWT.
func (s *Service) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, "", apperrors.Unauthorized("invalid credentials")
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if apperrors.Is(err, apperrors.KindNotFound) {
			return nil, "", apperrors.Unauthorized("invalid credentials")
		}
		return nil, "", err
	}
	if u.PasswordHash == nil {
		return nil, "", apperrors.Unauthorized("invalid credentials")
	}
	if err := hasher.Compare(*u.PasswordHash, password); err != nil {
		return nil, "", apperrors.Unauthorized("invalid credentials")
	}

	token, err := s.IssueJWT(u)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// Parse returns the user id and role encoded in the token. Used as middleware adapter.
func (s *Service) Parse(token string) (uuid.UUID, string, error) {
	claims, err := s.ParseJWT(token)
	if err != nil {
		return uuid.Nil, "", err
	}
	return claims.UserID, claims.Role, nil
}

// ParseFull returns full claims for downstream blacklist checks.
func (s *Service) ParseFull(token string) (*Claims, error) {
	return s.ParseJWT(token)
}

// Logout parses the token, computes its residual TTL, and adds the jti to
// the blacklist so subsequent requests bearing the same token are rejected.
// Returns Unauthorized for tokens that fail validation.
func (s *Service) Logout(ctx context.Context, token string) error {
	claims, err := s.ParseJWT(token)
	if err != nil {
		return err
	}
	jti := claims.JTI()
	exp := claims.Expiry()
	if jti == "" || exp.IsZero() {
		return apperrors.Unauthorized("invalid token")
	}
	ttl := time.Until(exp)
	if ttl <= 0 {
		return nil
	}
	return s.blacklist.Revoke(ctx, jti, ttl)
}

// IsRevoked reports whether the given jti is currently blacklisted.
func (s *Service) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.blacklist.IsRevoked(ctx, jti)
}

// Clock overrides the time source (for tests).
func (s *Service) Clock(fn func() time.Time) {
	if fn != nil {
		s.clock = fn
	}
}

func (s *Service) now() time.Time {
	if s.clock == nil {
		return time.Now()
	}
	return s.clock()
}
