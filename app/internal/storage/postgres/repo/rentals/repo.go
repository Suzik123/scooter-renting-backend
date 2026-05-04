package rentals

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

// CreateTx inserts a rental inside tx, mapping unique-violation constraint names to domain errors.
func (r *Repository) CreateTx(ctx context.Context, tx pgx.Tx, rental *models.Rental) error {
	var startTime any
	if !rental.StartTime.IsZero() {
		startTime = rental.StartTime
	}
	var status any
	if rental.Status != "" {
		status = rental.Status
	}
	row, err := r.q.WithTx(tx).CreateRental(ctx, sqlc.CreateRentalParams{
		UserID:       rental.UserID,
		ScooterID:    rental.ScooterID,
		PriceModelID: rental.PriceModelID,
		StartTime:    startTime,
		StartLat:     rental.StartLat,
		StartLon:     rental.StartLon,
		Status:       status,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			switch {
			case strings.Contains(pgErr.ConstraintName, "scooter"):
				return apperrors.ScooterUnavailable("scooter already has an active rental")
			case strings.Contains(pgErr.ConstraintName, "user"):
				return apperrors.Conflict("user already has active rental")
			default:
				return apperrors.Conflict("active rental conflict")
			}
		}
		return fmt.Errorf("rentals.CreateTx: %w", err)
	}
	*rental = fromSQLC(row)
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*models.Rental, error) {
	row, err := r.q.GetRental(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rental")
		}
		return nil, fmt.Errorf("rentals.Get: %w", err)
	}
	ret := fromSQLC(row)
	return &ret, nil
}

func (r *Repository) GetForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Rental, error) {
	row, err := r.q.WithTx(tx).GetRentalForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rental")
		}
		return nil, fmt.Errorf("rentals.GetForUpdateTx: %w", err)
	}
	ret := fromSQLC(row)
	return &ret, nil
}

func (r *Repository) EndTx(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
	endTime time.Time,
	endLat, endLon *decimal.Decimal,
	distanceM int,
	totalCost decimal.Decimal,
) (*models.Rental, error) {
	endTimeCopy := endTime
	row, err := r.q.WithTx(tx).EndRental(ctx, sqlc.EndRentalParams{
		EndTime:   &endTimeCopy,
		EndLat:    endLat,
		EndLon:    endLon,
		DistanceM: int32(distanceM),
		TotalCost: totalCost,
		RentalID:  id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.RentalAlreadyEnded("")
		}
		return nil, fmt.Errorf("rentals.EndTx: %w", err)
	}
	ret := fromSQLC(row)
	return &ret, nil
}

func (r *Repository) Cancel(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.CancelRental(ctx, id)
	if err != nil {
		return fmt.Errorf("rentals.Cancel: %w", err)
	}
	if n == 0 {
		return apperrors.Conflict("rental not active")
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Rental, int, error) {
	page = page.Clamp()
	total, err := r.q.CountRentalsByUser(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("rentals.ListByUser count: %w", err)
	}
	rows, err := r.q.ListRentalsByUser(ctx, sqlc.ListRentalsByUserParams{
		UserID: userID,
		Limit:  int32(page.Limit),
		Offset: int32(page.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("rentals.ListByUser: %w", err)
	}
	out := make([]models.Rental, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, int(total), nil
}

func fromSQLC(in sqlc.Rental) models.Rental {
	return models.Rental{
		ID:           in.RentalID,
		UserID:       in.UserID,
		ScooterID:    in.ScooterID,
		PriceModelID: in.PriceModelID,
		StartTime:    in.StartTime,
		EndTime:      in.EndTime,
		StartLat:     in.StartLat,
		StartLon:     in.StartLon,
		EndLat:       in.EndLat,
		EndLon:       in.EndLon,
		TotalCost:    in.TotalCost,
		Status:       in.Status,
		DistanceM:    int(in.DistanceM),
		CreatedAt:    in.CreatedAt,
		UpdatedAt:    in.UpdatedAt,
	}
}
