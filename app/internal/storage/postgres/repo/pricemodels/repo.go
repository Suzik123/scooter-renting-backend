package pricemodels

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
	Name           *string
	PricePerMinute *decimal.Decimal
	UnlockFee      *decimal.Decimal
	DailyCapSet    bool
	DailyCap       *decimal.Decimal
	Currency       *string
}

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

func (r *Repository) Create(ctx context.Context, pm *models.PriceModel) error {
	row, err := r.q.CreatePriceModel(ctx, sqlc.CreatePriceModelParams{
		Name:           pm.Name,
		UnlockFee:      pm.UnlockFee,
		PricePerMinute: pm.PricePerMinute,
		Currency:       pm.Currency,
		DailyCap:       pm.DailyCap,
	})
	if err != nil {
		return fmt.Errorf("pricemodels.Create: %w", err)
	}
	*pm = fromSQLC(row)
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*models.PriceModel, error) {
	row, err := r.q.GetPriceModel(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("price_model")
		}
		return nil, fmt.Errorf("pricemodels.Get: %w", err)
	}
	pm := fromSQLC(row)
	return &pm, nil
}

func (r *Repository) List(ctx context.Context, page models.Page) ([]models.PriceModel, int, error) {
	page = page.Clamp()
	total, err := r.q.CountPriceModels(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("pricemodels.List count: %w", err)
	}
	rows, err := r.q.ListPriceModels(ctx, sqlc.ListPriceModelsParams{Limit: int32(page.Limit), Offset: int32(page.Offset)})
	if err != nil {
		return nil, 0, fmt.Errorf("pricemodels.List: %w", err)
	}
	out := make([]models.PriceModel, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, int(total), nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.PriceModel, error) {
	row, err := r.q.UpdatePriceModel(ctx, sqlc.UpdatePriceModelParams{
		Name:           patch.Name,
		PricePerMinute: patch.PricePerMinute,
		UnlockFee:      patch.UnlockFee,
		DailyCapSet:    patch.DailyCapSet,
		DailyCap:       patch.DailyCap,
		Currency:       patch.Currency,
		PriceModelID:   id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("price_model")
		}
		return nil, fmt.Errorf("pricemodels.Update: %w", err)
	}
	pm := fromSQLC(row)
	return &pm, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.DeletePriceModel(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return apperrors.Conflict("price model is referenced by rentals")
		}
		return fmt.Errorf("pricemodels.Delete: %w", err)
	}
	if n == 0 {
		return apperrors.NotFound("price_model")
	}
	return nil
}

func fromSQLC(in sqlc.PriceModel) models.PriceModel {
	return models.PriceModel{
		ID:             in.PriceModelID,
		Name:           in.Name,
		UnlockFee:      in.UnlockFee,
		PricePerMinute: in.PricePerMinute,
		Currency:       in.Currency,
		DailyCap:       in.DailyCap,
		CreatedAt:      in.CreatedAt,
		UpdatedAt:      in.UpdatedAt,
	}
}
