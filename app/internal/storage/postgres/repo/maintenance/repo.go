package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

type Repository struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func New(q *sqlc.Queries, pool *pgxpool.Pool) *Repository {
	return &Repository{q: q, pool: pool}
}

func (r *Repository) Create(ctx context.Context, m *models.MaintenanceLog) error {
	var status any
	if m.Status != "" {
		status = m.Status
	}
	var repairCost *decimal.Decimal
	if m.RepairCost != nil {
		v := *m.RepairCost
		repairCost = &v
	}
	row, err := r.q.CreateMaintenance(ctx, sqlc.CreateMaintenanceParams{
		ScooterID:        m.ScooterID,
		TechnicianName:   m.TechnicianName,
		IssueDescription: m.IssueDescription,
		RepairCost:       repairCost,
		Status:           status,
	})
	if err != nil {
		return fmt.Errorf("maintenance.Create: %w", err)
	}
	*m = fromSQLC(row)
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error) {
	row, err := r.q.GetMaintenance(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("maintenance")
		}
		return nil, fmt.Errorf("maintenance.Get: %w", err)
	}
	m := fromSQLC(row)
	return &m, nil
}

func (r *Repository) Close(ctx context.Context, id uuid.UUID, endDate time.Time) (*models.MaintenanceLog, error) {
	endDateCopy := endDate
	row, err := r.q.CloseMaintenance(ctx, sqlc.CloseMaintenanceParams{EndDate: &endDateCopy, MaintenanceID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("maintenance not open")
		}
		return nil, fmt.Errorf("maintenance.Close: %w", err)
	}
	m := fromSQLC(row)
	return &m, nil
}

func (r *Repository) ListByScooter(ctx context.Context, scooterID uuid.UUID, page models.Page) ([]models.MaintenanceLog, int, error) {
	page = page.Clamp()
	total, err := r.q.CountMaintenanceByScooter(ctx, scooterID)
	if err != nil {
		return nil, 0, fmt.Errorf("maintenance.ListByScooter count: %w", err)
	}
	rows, err := r.q.ListMaintenanceByScooter(ctx, sqlc.ListMaintenanceByScooterParams{
		ScooterID: scooterID,
		Limit:     int32(page.Limit),
		Offset:    int32(page.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("maintenance.ListByScooter: %w", err)
	}
	out := make([]models.MaintenanceLog, len(rows))
	for i, row := range rows {
		out[i] = fromSQLC(row)
	}
	return out, int(total), nil
}

func fromSQLC(in sqlc.Maintenance) models.MaintenanceLog {
	return models.MaintenanceLog{
		ID:               in.MaintenanceID,
		ScooterID:        in.ScooterID,
		TechnicianName:   in.TechnicianName,
		IssueDescription: in.IssueDescription,
		RepairCost:       in.RepairCost,
		StartDate:        in.StartDate,
		EndDate:          in.EndDate,
		Status:           in.Status,
	}
}
