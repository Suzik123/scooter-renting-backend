package zones

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

type UpdatePatch struct {
	Name         *string
	CenterLat    *decimal.Decimal
	CenterLon    *decimal.Decimal
	RadiusMeters *int
	ZoneType     *string
}

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

func (r *Repository) Create(ctx context.Context, z *models.Zone) error {
	row, err := r.q.CreateZone(ctx, sqlc.CreateZoneParams{
		Name:         z.Name,
		CenterLat:    z.CenterLat,
		CenterLon:    z.CenterLon,
		RadiusMeters: int32(z.RadiusMeters),
		ZoneType:     nilIfEmpty(z.ZoneType),
	})
	if err != nil {
		return fmt.Errorf("zones.Create: %w", err)
	}
	*z = fromSQLCZone(row)
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*models.Zone, error) {
	row, err := r.q.GetZone(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("zone")
		}
		return nil, fmt.Errorf("zones.Get: %w", err)
	}
	z := fromSQLCZone(row)
	return &z, nil
}

func (r *Repository) List(ctx context.Context, page models.Page) ([]models.Zone, int, error) {
	page = page.Clamp()
	total, err := r.q.CountZones(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("zones.List count: %w", err)
	}
	rows, err := r.q.ListZones(ctx, sqlc.ListZonesParams{Limit: int32(page.Limit), Offset: int32(page.Offset)})
	if err != nil {
		return nil, 0, fmt.Errorf("zones.List: %w", err)
	}
	out := make([]models.Zone, len(rows))
	for i, row := range rows {
		out[i] = fromSQLCZone(row)
	}
	return out, int(total), nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.Zone, error) {
	var radius *int32
	if patch.RadiusMeters != nil {
		v := int32(*patch.RadiusMeters)
		radius = &v
	}
	row, err := r.q.UpdateZone(ctx, sqlc.UpdateZoneParams{
		Name:         patch.Name,
		CenterLat:    patch.CenterLat,
		CenterLon:    patch.CenterLon,
		RadiusMeters: radius,
		ZoneType:     patch.ZoneType,
		ZoneID:       id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("zone")
		}
		return nil, fmt.Errorf("zones.Update: %w", err)
	}
	z := fromSQLCZone(row)
	return &z, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.DeleteZone(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return apperrors.Conflict("zone is referenced by scooters or rentals")
		}
		return fmt.Errorf("zones.Delete: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("zone")
	}
	return nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func fromSQLCZone(in sqlc.Zone) models.Zone {
	return models.Zone{
		ID:           in.ZoneID,
		Name:         in.Name,
		CenterLat:    in.CenterLat,
		CenterLon:    in.CenterLon,
		RadiusMeters: int(in.RadiusMeters),
		ZoneType:     in.ZoneType,
		CreatedAt:    in.CreatedAt,
		UpdatedAt:    in.UpdatedAt,
	}
}
