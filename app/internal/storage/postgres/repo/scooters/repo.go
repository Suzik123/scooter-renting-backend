package scooters

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

type ListFilter struct {
	Status *string
	ZoneID *uuid.UUID
	Page   models.Page
}

type UpdatePatch struct {
	Model      *string
	Status     *string
	ZoneIDSet  bool
	ZoneID     *uuid.UUID
	BatteryPct *int
	LatSet     bool
	Lat        *decimal.Decimal
	LngSet     bool
	Lng        *decimal.Decimal
}

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

func (r *Repository) Create(ctx context.Context, s *models.Scooter) error {
	row, err := r.q.CreateScooter(ctx, sqlc.CreateScooterParams{
		Code:       s.Code,
		Model:      s.Model,
		BatteryPct: int32(s.BatteryPct),
		Status:     s.Status,
		ZoneID:     s.ZoneID,
		Lat:        s.Lat,
		Lng:        s.Lng,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return apperrors.Conflict("scooter code already exists")
		}
		return fmt.Errorf("scooters.Create: %w", err)
	}
	*s = fromSQLC(row)
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*models.Scooter, error) {
	row, err := r.q.GetScooter(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("scooter")
		}
		return nil, fmt.Errorf("scooters.Get: %w", err)
	}
	s := fromSQLC(row)
	return &s, nil
}

func (r *Repository) List(ctx context.Context, filter ListFilter) ([]models.Scooter, int, error) {
	page := filter.Page.Clamp()
	total, err := r.q.CountScooters(ctx, sqlc.CountScootersParams{Status: filter.Status, ZoneID: filter.ZoneID})
	if err != nil {
		return nil, 0, fmt.Errorf("scooters.List count: %w", err)
	}
	rows, err := r.q.ListScooters(ctx, sqlc.ListScootersParams{
		Status: filter.Status,
		ZoneID: filter.ZoneID,
		Off:    int32(page.Offset),
		Lim:    int32(page.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("scooters.List: %w", err)
	}
	out := make([]models.Scooter, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, int(total), nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.Scooter, error) {
	var battery *int32
	if patch.BatteryPct != nil {
		v := int32(*patch.BatteryPct)
		battery = &v
	}
	row, err := r.q.UpdateScooter(ctx, sqlc.UpdateScooterParams{
		Model:      patch.Model,
		Status:     patch.Status,
		ZoneIDSet:  patch.ZoneIDSet,
		ZoneID:     patch.ZoneID,
		BatteryPct: battery,
		LatSet:     patch.LatSet,
		Lat:        patch.Lat,
		LngSet:     patch.LngSet,
		Lng:        patch.Lng,
		ID:         id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("scooter")
		}
		return nil, fmt.Errorf("scooters.Update: %w", err)
	}
	s := fromSQLC(row)
	return &s, nil
}

func (r *Repository) Retire(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.RetireScooter(ctx, id)
	if err != nil {
		return fmt.Errorf("scooters.Retire: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("scooter")
	}
	return nil
}

func (r *Repository) FindNearby(ctx context.Context, lat, lng float64, radiusMeters int, limit int) ([]models.Scooter, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.q.FindNearbyScooters(ctx, sqlc.FindNearbyScootersParams{
		LngP:    lng,
		LatP:    lat,
		RadiusM: float64(radiusMeters),
		Lim:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("scooters.FindNearby: %w", err)
	}
	out := make([]models.Scooter, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, nil
}

// SetStatusTx transitions a scooter from fromStatus to toStatus inside tx.
func (r *Repository) SetStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, fromStatus, toStatus string) error {
	_, err := r.q.WithTx(tx).SetScooterStatus(ctx, sqlc.SetScooterStatusParams{
		ToStatus:   toStatus,
		ID:         id,
		FromStatus: fromStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.ScooterUnavailable("")
		}
		return fmt.Errorf("scooters.SetStatusTx: %w", err)
	}
	return nil
}

// GetTx reads a scooter inside tx.
func (r *Repository) GetTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Scooter, error) {
	row, err := r.q.WithTx(tx).GetScooter(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("scooter")
		}
		return nil, fmt.Errorf("scooters.GetTx: %w", err)
	}
	s := fromSQLC(row)
	return &s, nil
}

func fromSQLC(in sqlc.Scooter) models.Scooter {
	return models.Scooter{
		ID:         in.ID,
		Code:       in.Code,
		Model:      in.Model,
		BatteryPct: int(in.BatteryPct),
		Status:     in.Status,
		ZoneID:     in.ZoneID,
		Lat:        in.Lat,
		Lng:        in.Lng,
		CreatedAt:  in.CreatedAt,
		UpdatedAt:  in.UpdatedAt,
		DeletedAt:  in.DeletedAt,
	}
}
