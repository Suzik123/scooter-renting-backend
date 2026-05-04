package maintenance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

type Repo interface {
	Create(ctx context.Context, m *models.MaintenanceLog) error
	Get(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error)
	Close(ctx context.Context, id uuid.UUID, endDate time.Time) (*models.MaintenanceLog, error)
	ListByScooter(ctx context.Context, scooterID uuid.UUID, page models.Page) ([]models.MaintenanceLog, int, error)
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	ScooterID        uuid.UUID
	TechnicianName   string
	IssueDescription string
	RepairCost       *decimal.Decimal
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*models.MaintenanceLog, error) {
	desc := strings.TrimSpace(in.IssueDescription)
	if desc == "" {
		return nil, apperrors.Invalid("issue_description is required")
	}
	if in.RepairCost != nil && in.RepairCost.Sign() < 0 {
		return nil, apperrors.Invalid("repair_cost must be non-negative")
	}
	m := &models.MaintenanceLog{
		ScooterID:        in.ScooterID,
		TechnicianName:   strings.TrimSpace(in.TechnicianName),
		IssueDescription: desc,
		RepairCost:       in.RepairCost,
		Status:           models.MaintOpen,
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
