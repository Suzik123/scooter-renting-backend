package payments

import (
	"context"
	"errors"
	"fmt"
	"time"

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

type CreateInput struct {
	UserID            uuid.UUID
	RentalID          *uuid.UUID
	Amount            decimal.Decimal
	Currency          string
	PaymentMethod     string
	Status            string
	ProviderPaymentID *string
	FailureReason     *string
}

type AttachIntentInput struct {
	PaymentID         uuid.UUID
	ProviderPaymentID string
	Status            string
	FailureReason     *string
}

// CreateOfflineInput captures the input to CreateOffline.
type CreateOfflineInput struct {
	UserID         uuid.UUID
	RentalID       uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	ApproverID     uuid.UUID
	IdempotencyKey string
}

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*models.Payment, error) {
	row, err := r.q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		UserID:            in.UserID,
		RentalID:          in.RentalID,
		Amount:            in.Amount,
		ProviderPaymentID: in.ProviderPaymentID,
		Currency:          nilIfEmpty(in.Currency),
		PaymentMethod:     nilIfEmpty(in.PaymentMethod),
		Status:            nilIfEmpty(in.Status),
		FailureReason:     in.FailureReason,
	})
	if err != nil {
		return nil, fmt.Errorf("payments.Create: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	row, err := r.q.GetPaymentByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.GetByID: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

func (r *Repository) GetByProviderID(ctx context.Context, providerID string) (*models.Payment, error) {
	id := providerID
	row, err := r.q.GetPaymentByProviderID(ctx, &id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.GetByProviderID: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

// AttachIntent fills in the provider payment id (and updates status/failure)
// for a payment row that was inserted in 'pending' state before the Stripe
// call returned. Status may be empty to leave the existing value untouched.
func (r *Repository) AttachIntent(ctx context.Context, in AttachIntentInput) (*models.Payment, error) {
	pid := in.ProviderPaymentID
	row, err := r.q.AttachPaymentIntent(ctx, sqlc.AttachPaymentIntentParams{
		ProviderPaymentID: &pid,
		Status:            ptrIfNotEmpty(in.Status),
		FailureReason:     in.FailureReason,
		PaymentID:         in.PaymentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.AttachIntent: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

// MarkByIDFailed flips a payment row to 'failed' with a reason. Used when the
// Stripe call cannot return an intent id (e.g. network failure pre-create) so
// the row exists with provider_payment_id NULL.
func (r *Repository) MarkByIDFailed(ctx context.Context, id uuid.UUID, reason string) (*models.Payment, error) {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	row, err := r.q.MarkPaymentByIDFailed(ctx, sqlc.MarkPaymentByIDFailedParams{
		FailureReason: reasonPtr,
		PaymentID:     id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.MarkByIDFailed: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

func (r *Repository) MarkSucceeded(ctx context.Context, providerID string) (*models.Payment, error) {
	id := providerID
	row, err := r.q.MarkPaymentSucceeded(ctx, &id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.MarkSucceeded: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

func (r *Repository) MarkFailed(ctx context.Context, providerID, reason string) (*models.Payment, error) {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	pid := providerID
	row, err := r.q.MarkPaymentFailed(ctx, sqlc.MarkPaymentFailedParams{
		FailureReason:     reasonPtr,
		ProviderPaymentID: &pid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.MarkFailed: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Payment, int, error) {
	page = page.Clamp()
	total, err := r.q.CountPaymentsByUser(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("payments.ListByUser count: %w", err)
	}
	rows, err := r.q.ListPaymentsByUser(ctx, sqlc.ListPaymentsByUserParams{
		UserID: userID,
		Limit:  int32(page.Limit),
		Offset: int32(page.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("payments.ListByUser: %w", err)
	}
	out := make([]models.Payment, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, int(total), nil
}

// ListByUserSince returns payments newer than since (inclusive), paginated.
func (r *Repository) ListByUserSince(ctx context.Context, userID uuid.UUID, since time.Time, page models.Page) ([]models.Payment, int, error) {
	page = page.Clamp()
	total, err := r.q.CountPaymentsByUserSince(ctx, sqlc.CountPaymentsByUserSinceParams{
		UserID: userID,
		Since:  since,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("payments.ListByUserSince count: %w", err)
	}
	rows, err := r.q.ListPaymentsByUserSince(ctx, sqlc.ListPaymentsByUserSinceParams{
		UserID: userID,
		Since:  since,
		Limit:  int32(page.Limit),
		Offset: int32(page.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("payments.ListByUserSince: %w", err)
	}
	out := make([]models.Payment, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, int(total), nil
}

// CreateOffline inserts a new offline payment row with status='succeeded'.
// On idempotency-key conflict the existing row is returned with replay=true.
func (r *Repository) CreateOffline(ctx context.Context, in CreateOfflineInput) (*models.Payment, bool, error) {
	var idemArg *string
	if in.IdempotencyKey != "" {
		k := in.IdempotencyKey
		idemArg = &k
	}
	row, err := r.q.CreateOfflinePayment(ctx, sqlc.CreateOfflinePaymentParams{
		UserID:            in.UserID,
		RentalID:          &in.RentalID,
		Amount:            in.Amount,
		Currency:          in.Currency,
		OfflineApprovedBy: &in.ApproverID,
		IdempotencyKey:    idemArg,
	})
	if err != nil {
		// ON CONFLICT DO NOTHING returns ErrNoRows for the replay case.
		if errors.Is(err, pgx.ErrNoRows) && in.IdempotencyKey != "" {
			existing, gerr := r.GetByIdempotencyKey(ctx, in.IdempotencyKey)
			if gerr != nil {
				return nil, false, fmt.Errorf("payments.CreateOffline replay lookup: %w", gerr)
			}
			return existing, true, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && in.IdempotencyKey != "" {
			existing, gerr := r.GetByIdempotencyKey(ctx, in.IdempotencyKey)
			if gerr != nil {
				return nil, false, fmt.Errorf("payments.CreateOffline replay lookup: %w", gerr)
			}
			return existing, true, nil
		}
		return nil, false, fmt.Errorf("payments.CreateOffline: %w", err)
	}
	p := fromSQLC(row)
	return &p, false, nil
}

// GetByIdempotencyKey returns the payment that owns the given idempotency key
// or a NotFound apperror.
func (r *Repository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error) {
	if key == "" {
		return nil, apperrors.NotFound("payment")
	}
	k := key
	row, err := r.q.GetPaymentByIdempotencyKey(ctx, &k)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("payment")
		}
		return nil, fmt.Errorf("payments.GetByIdempotencyKey: %w", err)
	}
	p := fromSQLC(row)
	return &p, nil
}

func (r *Repository) HasUnpaidRentals(ctx context.Context, userID uuid.UUID) (bool, error) {
	v, err := r.q.HasUnpaidRentals(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("payments.HasUnpaidRentals: %w", err)
	}
	return v, nil
}

// InsertWebhookEvent returns true when the event was inserted; false when duplicate.
func (r *Repository) InsertWebhookEvent(ctx context.Context, eventID, eventType string, payload []byte) (bool, error) {
	_, err := r.q.InsertWebhookEvent(ctx, sqlc.InsertWebhookEventParams{
		EventID: eventID,
		Type:    eventType,
		Payload: payload,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("payments.InsertWebhookEvent: %w", err)
	}
	return true, nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func fromSQLC(in sqlc.Payment) models.Payment {
	return models.Payment{
		ID:                in.PaymentID,
		UserID:            in.UserID,
		RentalID:          in.RentalID,
		Amount:            in.Amount,
		Currency:          in.Currency,
		PaymentMethod:     in.PaymentMethod,
		Status:            in.Status,
		ProviderPaymentID: in.ProviderPaymentID,
		FailureReason:     in.FailureReason,
		TransactionDate:   in.TransactionDate,
		UpdatedAt:         in.UpdatedAt,
		OfflineApprovedBy: in.OfflineApprovedBy,
		OfflineApprovedAt: in.OfflineApprovedAt,
		IdempotencyKey:    in.IdempotencyKey,
	}
}
