package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
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
	users UserRepo
	cfg   *config.Config
	clock func() time.Time
}

func New(cfg *config.Config, users UserRepo) *Service {
	return &Service{
		users: users,
		cfg:   cfg,
		clock: time.Now,
	}
}

// Register creates a new user with role "user" and returns the user with a freshly issued JWT.
func (s *Service) Register(ctx context.Context, email, name, password, phone string) (*models.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	if email == "" || name == "" || password == "" {
		return nil, "", apperrors.Invalid("email, name and password are required")
	}

	hash, err := hasher.Hash(password, s.cfg.Bcrypt.Cost)
	if err != nil {
		return nil, "", apperrors.Internal("hash password")
	}

	u := &models.User{
		Email:        email,
		Name:         name,
		PasswordHash: &hash,
		Role:         models.RoleUser,
	}
	if phone != "" {
		u.Phone = &phone
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

