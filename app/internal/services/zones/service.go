package zones

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	zonesrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/zones"
)

type Repo interface {
	Create(ctx context.Context, z *models.Zone) error
	Get(ctx context.Context, id uuid.UUID) (*models.Zone, error)
	List(ctx context.Context, page models.Page) ([]models.Zone, int, error)
	Update(ctx context.Context, id uuid.UUID, patch zonesrepo.UpdatePatch) (*models.Zone, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name         string
	CenterLat    decimal.Decimal
	CenterLon    decimal.Decimal
	RadiusMeters int
	ZoneType     string
}

type UpdatePatch struct {
	Name         *string
	CenterLat    *decimal.Decimal
	CenterLon    *decimal.Decimal
	RadiusMeters *int
	ZoneType     *string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*models.Zone, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperrors.Invalid("name is required")
	}
	if in.RadiusMeters <= 0 {
		return nil, apperrors.Invalid("radius_meters must be positive")
	}
	zoneType := strings.TrimSpace(in.ZoneType)
	if zoneType == "" {
		zoneType = models.ZoneTypeService
	}
	switch zoneType {
	case models.ZoneTypeService, models.ZoneTypeNoPark, models.ZoneTypeReducedSpeed:
	default:
		return nil, apperrors.Invalid("invalid zone_type")
	}
	z := &models.Zone{
		Name:         name,
		CenterLat:    in.CenterLat,
		CenterLon:    in.CenterLon,
		RadiusMeters: in.RadiusMeters,
		ZoneType:     zoneType,
	}
	if err := s.repo.Create(ctx, z); err != nil {
		return nil, err
	}
	return z, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.Zone, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, page models.Page) ([]models.Zone, int, error) {
	return s.repo.List(ctx, page)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.Zone, error) {
	if patch.ZoneType != nil {
		switch *patch.ZoneType {
		case models.ZoneTypeService, models.ZoneTypeNoPark, models.ZoneTypeReducedSpeed:
		default:
			return nil, apperrors.Invalid("invalid zone_type")
		}
	}
	if patch.RadiusMeters != nil && *patch.RadiusMeters <= 0 {
		return nil, apperrors.Invalid("radius_meters must be positive")
	}
	return s.repo.Update(ctx, id, zonesrepo.UpdatePatch{
		Name:         patch.Name,
		CenterLat:    patch.CenterLat,
		CenterLon:    patch.CenterLon,
		RadiusMeters: patch.RadiusMeters,
		ZoneType:     patch.ZoneType,
	})
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
