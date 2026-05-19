package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

type UpdatePatch struct {
	FirstName   *string
	LastName    *string
	PhoneNumber *string
}

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

func (r *Repository) Create(ctx context.Context, u *models.User) error {
	row, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Email:         u.Email,
		PhoneNumber:   u.PhoneNumber,
		PasswordHash:  u.PasswordHash,
		OauthProvider: u.OAuthProvider,
		OauthSubject:  u.OAuthSubject,
		Role:          u.Role,
		Status:        nilIfEmpty(u.Status),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return apperrors.Conflict("email already exists")
		}
		return fmt.Errorf("users.Create: %w", err)
	}
	*u = fromSQLCUser(row)
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("users.GetByID: %w", err)
	}
	u := fromSQLCUser(row)
	return &u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("users.GetByEmail: %w", err)
	}
	u := fromSQLCUser(row)
	return &u, nil
}

func (r *Repository) GetByOAuth(ctx context.Context, provider, subject string) (*models.User, error) {
	row, err := r.q.GetUserByOAuth(ctx, sqlc.GetUserByOAuthParams{
		OauthProvider: &provider,
		OauthSubject:  &subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("users.GetByOAuth: %w", err)
	}
	u := fromSQLCUser(row)
	return &u, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.User, error) {
	row, err := r.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		FirstName:   patch.FirstName,
		LastName:    patch.LastName,
		PhoneNumber: patch.PhoneNumber,
		UserID:      id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("users.Update: %w", err)
	}
	u := fromSQLCUser(row)
	return &u, nil
}

func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteUser(ctx, id)
	if err != nil {
		return fmt.Errorf("users.SoftDelete: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("user")
	}
	return nil
}

func (r *Repository) SetRole(ctx context.Context, id uuid.UUID, role string) error {
	n, err := r.q.SetUserRole(ctx, sqlc.SetUserRoleParams{Role: role, UserID: id})
	if err != nil {
		return fmt.Errorf("users.SetRole: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("user")
	}
	return nil
}

func (r *Repository) LinkOAuth(ctx context.Context, id uuid.UUID, provider, subject string) (*models.User, error) {
	row, err := r.q.LinkUserOAuth(ctx, sqlc.LinkUserOAuthParams{
		OauthProvider: &provider,
		OauthSubject:  &subject,
		UserID:        id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, apperrors.Conflict("oauth identity already linked")
		}
		return nil, fmt.Errorf("users.LinkOAuth: %w", err)
	}
	u := fromSQLCUser(row)
	return &u, nil
}

// ResetPassword overwrites the user's password hash and bumps last_logout_at
// to NOW(). The JWT middleware compares each token's iat against
// last_logout_at on every request, so this kills every existing session for
// the user in addition to setting the new credential.
func (r *Repository) ResetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	n, err := r.q.ResetUserPassword(ctx, sqlc.ResetUserPasswordParams{
		PasswordHash: &passwordHash,
		UserID:       id,
	})
	if err != nil {
		return fmt.Errorf("users.ResetPassword: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("user")
	}
	return nil
}

func (r *Repository) SetStripeCustomerID(ctx context.Context, id uuid.UUID, customerID string) (*models.User, error) {
	row, err := r.q.SetStripeCustomerID(ctx, sqlc.SetStripeCustomerIDParams{
		StripeCustomerID: &customerID,
		UserID:           id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("users.SetStripeCustomerID: %w", err)
	}
	u := fromSQLCUser(row)
	return &u, nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func fromSQLCUser(in sqlc.User) models.User {
	return models.User{
		ID:               in.UserID,
		FirstName:        in.FirstName,
		LastName:         in.LastName,
		Email:            in.Email,
		PhoneNumber:      in.PhoneNumber,
		RegistrationDate: in.RegistrationDate,
		Status:           in.Status,
		Role:             in.Role,
		PasswordHash:     in.PasswordHash,
		OAuthProvider:    in.OauthProvider,
		OAuthSubject:     in.OauthSubject,
		StripeCustomerID: in.StripeCustomerID,
		UpdatedAt:        in.UpdatedAt,
	}
}
