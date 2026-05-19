// Package passwordresets owns persistence for password reset tokens.
// Tokens are stored as bytea hashes; raw tokens never touch the DB.
package passwordresets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

// Token is the domain-level view of a password_reset_tokens row.
type Token struct {
	TokenID   uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Repository is a thin facade over the sqlc-generated queries. The pool is
// kept available for future query-runner switching (e.g. inside a tx).
type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// New constructs a Repository.
func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

// Create inserts a fresh token row and returns the persisted view.
func (r *Repository) Create(ctx context.Context, userID uuid.UUID, hash []byte, expiresAt time.Time) (*Token, error) {
	row, err := r.q.CreatePasswordResetToken(ctx, sqlc.CreatePasswordResetTokenParams{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("password_reset.Create: %w", err)
	}
	t := fromSQLC(row)
	return &t, nil
}

// GetByHash looks up a token row by its sha256 hash. Returns NotFound when
// no matching row exists; callers should not distinguish "wrong token" from
// "no such token" so the same NotFound path covers both.
func (r *Repository) GetByHash(ctx context.Context, hash []byte) (*Token, error) {
	row, err := r.q.GetPasswordResetTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("password reset token")
		}
		return nil, fmt.Errorf("password_reset.GetByHash: %w", err)
	}
	t := fromSQLC(row)
	return &t, nil
}

// InvalidateAllForUser marks every still-active token for the user as used.
// Used by Request to ensure a fresh token always replaces stale ones, so an
// attacker who scraped an old token cannot ride a freshly-requested reset.
func (r *Repository) InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.q.InvalidatePasswordResetTokensForUser(ctx, userID); err != nil {
		return fmt.Errorf("password_reset.InvalidateAllForUser: %w", err)
	}
	return nil
}

// MarkUsed flips a single token's used_at to NOW(). Returns NotFound when
// the row was already used (or does not exist). Used during Confirm.
func (r *Repository) MarkUsed(ctx context.Context, tokenID uuid.UUID) error {
	n, err := r.q.MarkPasswordResetTokenUsed(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("password_reset.MarkUsed: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("password reset token")
	}
	return nil
}

func fromSQLC(in sqlc.PasswordResetToken) Token {
	return Token{
		TokenID:   in.TokenID,
		UserID:    in.UserID,
		TokenHash: in.TokenHash,
		ExpiresAt: in.ExpiresAt,
		UsedAt:    in.UsedAt,
		CreatedAt: in.CreatedAt,
	}
}
