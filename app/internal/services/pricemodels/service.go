package pricemodels

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	pmrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/pricemodels"
)

type Repo interface {
	Create(ctx context.Context, pm *models.PriceModel) error
	Get(ctx context.Context, id uuid.UUID) (*models.PriceModel, error)
	List(ctx context.Context, page models.Page) ([]models.PriceModel, int, error)
	Update(ctx context.Context, id uuid.UUID, patch pmrepo.UpdatePatch) (*models.PriceModel, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name          string
	PerMinuteRate decimal.Decimal
	UnlockFee     decimal.Decimal
	DailyCap      *decimal.Decimal
	Currency      string
}

type UpdatePatch struct {
	Name          *string
	PerMinuteRate *decimal.Decimal
	UnlockFee     *decimal.Decimal
	DailyCapSet   bool
	DailyCap      *decimal.Decimal
	Currency      *string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*models.PriceModel, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperrors.Invalid("name is required")
	}
	if in.PerMinuteRate.Sign() < 0 || in.UnlockFee.Sign() < 0 {
		return nil, apperrors.Invalid("rates must be non-negative")
	}
	if in.DailyCap != nil && in.DailyCap.Sign() < 0 {
		return nil, apperrors.Invalid("daily_cap must be non-negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "PLN"
	}
	if len(currency) != 3 {
		return nil, apperrors.Invalid("currency must be a 3-letter code")
	}

	pm := &models.PriceModel{
		Name:          name,
		PerMinuteRate: in.PerMinuteRate,
		UnlockFee:     in.UnlockFee,
		DailyCap:      in.DailyCap,
		Currency:      currency,
	}
	if err := s.repo.Create(ctx, pm); err != nil {
		return nil, err
	}
	return pm, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.PriceModel, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, page models.Page) ([]models.PriceModel, int, error) {
	return s.repo.List(ctx, page)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.PriceModel, error) {
	if patch.Currency != nil {
		v := strings.ToUpper(strings.TrimSpace(*patch.Currency))
		if len(v) != 3 {
			return nil, apperrors.Invalid("currency must be a 3-letter code")
		}
		patch.Currency = &v
	}
	return s.repo.Update(ctx, id, pmrepo.UpdatePatch{
		Name:          patch.Name,
		PerMinuteRate: patch.PerMinuteRate,
		UnlockFee:     patch.UnlockFee,
		DailyCapSet:   patch.DailyCapSet,
		DailyCap:      patch.DailyCap,
		Currency:      patch.Currency,
	})
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
