package scooters

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	sc "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/scooters"
)

const maxNearbyRadiusMeters = 20000

// Repo is the subset of the scooters repository this service uses.
type Repo interface {
	Create(ctx context.Context, s *models.Scooter) error
	Get(ctx context.Context, id uuid.UUID) (*models.Scooter, error)
	List(ctx context.Context, filter sc.ListFilter) ([]models.Scooter, int, error)
	Update(ctx context.Context, id uuid.UUID, patch sc.UpdatePatch) (*models.Scooter, error)
	Retire(ctx context.Context, id uuid.UUID) error
	FindNearby(ctx context.Context, lat, lng float64, radiusMeters int, limit int) ([]models.Scooter, error)
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

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

func (s *Service) Create(ctx context.Context, code, model string, zoneID *uuid.UUID, lat, lng *decimal.Decimal, batteryPct int) (*models.Scooter, error) {
	code = strings.TrimSpace(code)
	model = strings.TrimSpace(model)
	if code == "" || model == "" {
		return nil, apperrors.Invalid("code and model are required")
	}
	if batteryPct < 0 || batteryPct > 100 {
		return nil, apperrors.Invalid("battery_pct must be in [0,100]")
	}
	if (lat == nil) != (lng == nil) {
		return nil, apperrors.Invalid("lat and lng must be provided together")
	}
	sc := &models.Scooter{
		Code:       code,
		Model:      model,
		BatteryPct: batteryPct,
		Status:     models.ScooterAvailable,
		ZoneID:     zoneID,
		Lat:        lat,
		Lng:        lng,
	}
	if err := s.repo.Create(ctx, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.Scooter, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]models.Scooter, int, error) {
	return s.repo.List(ctx, sc.ListFilter{
		Status: filter.Status,
		ZoneID: filter.ZoneID,
		Page:   filter.Page,
	})
}

func (s *Service) Nearby(ctx context.Context, lat, lng float64, radiusMeters, limit int) ([]models.Scooter, error) {
	if lat < -90 || lat > 90 {
		return nil, apperrors.Invalid("lat out of range")
	}
	if lng < -180 || lng > 180 {
		return nil, apperrors.Invalid("lng out of range")
	}
	if radiusMeters <= 0 || radiusMeters > maxNearbyRadiusMeters {
		return nil, apperrors.Invalid("radius must be 1..20000 meters")
	}
	if limit <= 0 {
		limit = 20
	}
	return s.repo.FindNearby(ctx, lat, lng, radiusMeters, limit)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.Scooter, error) {
	if patch.BatteryPct != nil {
		if v := *patch.BatteryPct; v < 0 || v > 100 {
			return nil, apperrors.Invalid("battery_pct must be in [0,100]")
		}
	}
	if patch.Status != nil {
		switch *patch.Status {
		case models.ScooterAvailable, models.ScooterRented, models.ScooterMaintenance:
		default:
			return nil, apperrors.Invalid("invalid status")
		}
	}
	return s.repo.Update(ctx, id, sc.UpdatePatch{
		Model:      patch.Model,
		Status:     patch.Status,
		ZoneIDSet:  patch.ZoneIDSet,
		ZoneID:     patch.ZoneID,
		BatteryPct: patch.BatteryPct,
		LatSet:     patch.LatSet,
		Lat:        patch.Lat,
		LngSet:     patch.LngSet,
		Lng:        patch.Lng,
	})
}

func (s *Service) Retire(ctx context.Context, id uuid.UUID) error {
	return s.repo.Retire(ctx, id)
}
