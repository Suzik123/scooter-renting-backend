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
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

type UpdatePatch struct {
	Name  *string
	Phone *string
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
		Email:         u.Email,
		Name:          u.Name,
		Phone:         u.Phone,
		PasswordHash:  u.PasswordHash,
		OauthID:       u.OAuthID,
		Role:          u.Role,
		WalletBalance: u.WalletBalance,
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

func (r *Repository) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.User, error) {
	row, err := r.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		Name:  patch.Name,
		Phone: patch.Phone,
		ID:    id,
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

func (r *Repository) AdjustWallet(ctx context.Context, id uuid.UUID, delta decimal.Decimal) (decimal.Decimal, error) {
	balance, err := r.q.AdjustWallet(ctx, sqlc.AdjustWalletParams{Delta: delta, ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, apperrors.Insufficient("")
		}
		return decimal.Zero, fmt.Errorf("users.AdjustWallet: %w", err)
	}
	return balance, nil
}

func (r *Repository) GetWallet(ctx context.Context, id uuid.UUID) (decimal.Decimal, error) {
	balance, err := r.q.GetWallet(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, apperrors.NotFound("user")
		}
		return decimal.Zero, fmt.Errorf("users.GetWallet: %w", err)
	}
	return balance, nil
}

func (r *Repository) SetRole(ctx context.Context, id uuid.UUID, role string) error {
	n, err := r.q.SetUserRole(ctx, sqlc.SetUserRoleParams{Role: role, ID: id})
	if err != nil {
		return fmt.Errorf("users.SetRole: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("user")
	}
	return nil
}

func fromSQLCUser(in sqlc.User) models.User {
	return models.User{
		ID:            in.ID,
		Email:         in.Email,
		Name:          in.Name,
		Phone:         in.Phone,
		PasswordHash:  in.PasswordHash,
		OAuthID:       in.OauthID,
		Role:          in.Role,
		WalletBalance: in.WalletBalance,
		CreatedAt:     in.CreatedAt,
		UpdatedAt:     in.UpdatedAt,
		DeletedAt:     in.DeletedAt,
	}
}
