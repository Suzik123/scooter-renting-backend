package zones

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

type Repo interface {
	Create(ctx context.Context, z *models.Zone) error
	Get(ctx context.Context, id uuid.UUID) (*models.Zone, error)
	List(ctx context.Context, page models.Page) ([]models.Zone, int, error)
	Update(ctx context.Context, id uuid.UUID, name *string, boundary *string) (*models.Zone, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name string, boundary *string) (*models.Zone, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperrors.Invalid("name is required")
	}
	z := &models.Zone{Name: name, Boundary: boundary}
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

func (s *Service) Update(ctx context.Context, id uuid.UUID, name *string, boundary *string) (*models.Zone, error) {
	return s.repo.Update(ctx, id, name, boundary)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
