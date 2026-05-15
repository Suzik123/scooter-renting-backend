package maintenance_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/services/maintenance"
)

type fakeRepo struct {
	created *models.MaintenanceLog
}

func (f *fakeRepo) Create(_ context.Context, m *models.MaintenanceLog) error {
	m.ID = uuid.New()
	f.created = m
	return nil
}
func (f *fakeRepo) Get(_ context.Context, _ uuid.UUID) (*models.MaintenanceLog, error) {
	return f.created, nil
}
func (f *fakeRepo) Close(_ context.Context, _ uuid.UUID, _ time.Time) (*models.MaintenanceLog, error) {
	return f.created, nil
}
func (f *fakeRepo) ListByScooter(_ context.Context, _ uuid.UUID, _ models.Page) ([]models.MaintenanceLog, int, error) {
	return nil, 0, nil
}

func TestCreate_RequiresIssue(t *testing.T) {
	s := maintenance.New(&fakeRepo{})
	_, err := s.Create(context.Background(), maintenance.CreateInput{ScooterID: uuid.New(), IssueDescription: "  "})
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestCreate_RejectsNegativeRepairCost(t *testing.T) {
	c := decimal.NewFromInt(-1)
	s := maintenance.New(&fakeRepo{})
	_, err := s.Create(context.Background(), maintenance.CreateInput{
		ScooterID:        uuid.New(),
		IssueDescription: "broken",
		RepairCost:       &c,
	})
	require.Error(t, err)
}

func TestCreate_HappyPath(t *testing.T) {
	s := maintenance.New(&fakeRepo{})
	m, err := s.Create(context.Background(), maintenance.CreateInput{
		ScooterID:        uuid.New(),
		IssueDescription: "broken",
	})
	require.NoError(t, err)
	assert.Equal(t, models.MaintOpen, m.Status)
}

func TestClose_PassesThrough(t *testing.T) {
	s := maintenance.New(&fakeRepo{created: &models.MaintenanceLog{}})
	_, err := s.Close(context.Background(), uuid.New())
	require.NoError(t, err)
}
