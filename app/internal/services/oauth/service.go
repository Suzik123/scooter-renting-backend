package oauth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	googleclient "github.com/uniscoot/scooter-renting-backend/app/clients/google"
	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
)

// UserRepo is the subset of the users repository used here.
type UserRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByOAuth(ctx context.Context, provider, subject string) (*models.User, error)
	Create(ctx context.Context, u *models.User) error
	LinkOAuth(ctx context.Context, id uuid.UUID, provider, subject string) (*models.User, error)
}

// GoogleClient verifies a Google id_token.
type GoogleClient interface {
	VerifyIDToken(ctx context.Context, rawIDToken string) (*googleclient.Claims, error)
}

// Service handles "Sign in with Google" flows.
type Service struct {
	users  UserRepo
	auth   *authsvc.Service
	google GoogleClient
	cfg    *config.Config
	clock  func() time.Time
}

func New(users UserRepo, auth *authsvc.Service, google GoogleClient, cfg *config.Config) *Service {
	return &Service{
		users:  users,
		auth:   auth,
		google: google,
		cfg:    cfg,
		clock:  time.Now,
	}
}

// VerifyAndLogin verifies a Google id_token and returns the user with a
// freshly-issued JWT. Existing users are matched by oauth identity first,
// then by email. Otherwise a new user is provisioned without a password.
func (s *Service) VerifyAndLogin(ctx context.Context, idToken string) (*models.User, string, error) {
	claims, err := s.google.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, "", apperrors.Unauthorized("invalid google id token")
	}

	if hd := strings.TrimSpace(s.cfg.Google.HostedDomain); hd != "" && claims.HostedDomain != hd {
		return nil, "", apperrors.Unauthorized("hosted domain not allowed")
	}
	if !claims.EmailVerified {
		return nil, "", apperrors.Unauthorized("email not verified by google")
	}
	if claims.Subject == "" || claims.Email == "" {
		return nil, "", apperrors.Unauthorized("missing required google claims")
	}

	const provider = "google"

	if u, err := s.users.GetByOAuth(ctx, provider, claims.Subject); err == nil {
		token, err := s.auth.IssueJWT(u)
		if err != nil {
			return nil, "", err
		}
		return u, token, nil
	} else if !apperrors.Is(err, apperrors.KindNotFound) {
		return nil, "", err
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if u, err := s.users.GetByEmail(ctx, email); err == nil {
		// If the existing account is already bound to a different OAuth
		// identity (provider+subject), refuse to silently re-bind it.
		// This blocks an account-takeover where another user with the
		// same email tries to attach their Google identity to a record
		// that belongs to a previously-linked SSO account.
		linkedProvider := ""
		if u.OAuthProvider != nil {
			linkedProvider = *u.OAuthProvider
		}
		linkedSubject := ""
		if u.OAuthSubject != nil {
			linkedSubject = *u.OAuthSubject
		}
		if linkedProvider != "" && linkedSubject != "" &&
			(linkedProvider != provider || linkedSubject != claims.Subject) {
			return nil, "", apperrors.Conflict("account_already_linked")
		}
		if linkedProvider == "" {
			linked, err := s.users.LinkOAuth(ctx, u.ID, provider, claims.Subject)
			if err != nil {
				return nil, "", err
			}
			u = linked
		}
		token, err := s.auth.IssueJWT(u)
		if err != nil {
			return nil, "", err
		}
		return u, token, nil
	} else if !apperrors.Is(err, apperrors.KindNotFound) {
		return nil, "", err
	}

	prov := provider
	sub := claims.Subject
	first := strings.TrimSpace(claims.GivenName)
	if first == "" {
		first = strings.TrimSpace(claims.Name)
	}
	if first == "" {
		first = email
	}

	newUser := &models.User{
		Email:         email,
		FirstName:     first,
		LastName:      strings.TrimSpace(claims.FamilyName),
		Role:          models.RoleUser,
		Status:        models.UserActive,
		OAuthProvider: &prov,
		OAuthSubject:  &sub,
	}
	if err := s.users.Create(ctx, newUser); err != nil {
		return nil, "", err
	}
	token, err := s.auth.IssueJWT(newUser)
	if err != nil {
		return nil, "", err
	}
	return newUser, token, nil
}
