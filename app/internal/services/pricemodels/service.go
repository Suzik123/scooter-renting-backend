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
	Name           string
	PricePerMinute decimal.Decimal
	UnlockFee      decimal.Decimal
	DailyCap       *decimal.Decimal
	Currency       string
	// Force skips the recommended-band check (0.05..2.00 / 0..10) but the
	// non-negative check remains. Set via ?force=true at the handler layer
	// for admins overriding intentionally.
	Force bool
}

type UpdatePatch struct {
	Name           *string
	PricePerMinute *decimal.Decimal
	UnlockFee      *decimal.Decimal
	DailyCapSet    bool
	DailyCap       *decimal.Decimal
	Currency       *string
	// Force skips the recommended-band check on Update. See CreateInput.Force.
	Force bool
}

// Recommended bands enforced when Force is false. The intent is a safety
// guard against seeding errors (e.g. confusing dollars and per-minute rates),
// not a hard product limit.
var (
	minPricePerMinute = decimal.NewFromFloat(0.05)
	maxPricePerMinute = decimal.NewFromFloat(2.00)
	maxUnlockFee      = decimal.NewFromFloat(10.00)
)

// checkPricingBand returns an Invalid apperror when ppm/unlock fall outside
// the recommended band. Non-negative checks are caller-enforced.
func checkPricingBand(ppm, unlock decimal.Decimal) error {
	if ppm.LessThan(minPricePerMinute) || ppm.GreaterThan(maxPricePerMinute) {
		return apperrors.Invalid("price_per_minute out of recommended band 0.05-2.00; pass force=true to override")
	}
	if unlock.GreaterThan(maxUnlockFee) {
		return apperrors.Invalid("unlock_fee out of recommended band 0-10.00; pass force=true to override")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*models.PriceModel, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperrors.Invalid("name is required")
	}
	if in.PricePerMinute.Sign() < 0 || in.UnlockFee.Sign() < 0 {
		return nil, apperrors.Invalid("rates must be non-negative")
	}
	if in.DailyCap != nil && in.DailyCap.Sign() < 0 {
		return nil, apperrors.Invalid("daily_cap must be non-negative")
	}
	if !in.Force {
		if err := checkPricingBand(in.PricePerMinute, in.UnlockFee); err != nil {
			return nil, err
		}
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "USD"
	}
	if len(currency) != 3 {
		return nil, apperrors.Invalid("currency must be a 3-letter code")
	}

	pm := &models.PriceModel{
		Name:           name,
		PricePerMinute: in.PricePerMinute,
		UnlockFee:      in.UnlockFee,
		DailyCap:       in.DailyCap,
		Currency:       currency,
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
	if patch.PricePerMinute != nil && patch.PricePerMinute.Sign() < 0 {
		return nil, apperrors.Invalid("price_per_minute must be non-negative")
	}
	if patch.UnlockFee != nil && patch.UnlockFee.Sign() < 0 {
		return nil, apperrors.Invalid("unlock_fee must be non-negative")
	}
	if patch.DailyCapSet && patch.DailyCap != nil && patch.DailyCap.Sign() < 0 {
		return nil, apperrors.Invalid("daily_cap must be non-negative")
	}
	if !patch.Force {
		// Only run the band check on the fields the patch actually touches.
		// We use sane defaults (the midpoint of the band) for fields the
		// caller did not provide so the existing values are not held against
		// them.
		ppm := decimal.NewFromFloat(1.0)
		if patch.PricePerMinute != nil {
			ppm = *patch.PricePerMinute
		}
		unlock := decimal.Zero
		if patch.UnlockFee != nil {
			unlock = *patch.UnlockFee
		}
		// Only enforce a band check on fields that were actually provided.
		if patch.PricePerMinute != nil || patch.UnlockFee != nil {
			if patch.PricePerMinute == nil {
				ppm = decimal.NewFromFloat(0.20)
			}
			if err := checkPricingBand(ppm, unlock); err != nil {
				return nil, err
			}
		}
	}
	return s.repo.Update(ctx, id, pmrepo.UpdatePatch{
		Name:           patch.Name,
		PricePerMinute: patch.PricePerMinute,
		UnlockFee:      patch.UnlockFee,
		DailyCapSet:    patch.DailyCapSet,
		DailyCap:       patch.DailyCap,
		Currency:       patch.Currency,
	})
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
