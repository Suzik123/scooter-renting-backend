package maintenance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

type Repo interface {
	Create(ctx context.Context, m *models.MaintenanceLog) error
	Get(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error)
	Close(ctx context.Context, id uuid.UUID, closedAt time.Time) (*models.MaintenanceLog, error)
	ListByScooter(ctx context.Context, scooterID uuid.UUID, page models.Page) ([]models.MaintenanceLog, int, error)
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, scooterID uuid.UUID, description string, technicianID *uuid.UUID) (*models.MaintenanceLog, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, apperrors.Invalid("description is required")
	}
	m := &models.MaintenanceLog{
		ScooterID:    scooterID,
		Description:  description,
		TechnicianID: technicianID,
		Status:       models.MaintOpen,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) Close(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error) {
	return s.repo.Close(ctx, id, time.Now().UTC())
}

func (s *Service) ListByScooter(ctx context.Context, scooterID uuid.UUID, page models.Page) ([]models.MaintenanceLog, int, error) {
	return s.repo.ListByScooter(ctx, scooterID, page)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error) {
	return s.repo.Get(ctx, id)
}
