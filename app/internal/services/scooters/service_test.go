package scooters_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/services/scooters"
	sc "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/scooters"
)

type fakeRepo struct {
	created *models.Scooter
}

func (f *fakeRepo) Create(_ context.Context, s *models.Scooter) error {
	s.ID = uuid.New()
	f.created = s
	return nil
}
func (f *fakeRepo) Get(_ context.Context, _ uuid.UUID) (*models.Scooter, error) {
	return f.created, nil
}
func (f *fakeRepo) List(_ context.Context, _ sc.ListFilter) ([]models.Scooter, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Update(_ context.Context, _ uuid.UUID, _ sc.UpdatePatch) (*models.Scooter, error) {
	return f.created, nil
}
func (f *fakeRepo) Retire(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeRepo) FindNearby(_ context.Context, _, _ float64, _, _ int) ([]models.Scooter, error) {
	return nil, nil
}

func TestCreate_RejectsBlankQR(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	_, err := s.Create(context.Background(), " ", "M", nil, nil, nil, 50)
	require.Error(t, err)
}

func TestCreate_RejectsBadBattery(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	_, err := s.Create(context.Background(), "qr", "M", nil, nil, nil, 200)
	require.Error(t, err)
}

func TestCreate_RejectsLatWithoutLng(t *testing.T) {
	lat := decimal.NewFromFloat(10.0)
	s := scooters.New(&fakeRepo{})
	_, err := s.Create(context.Background(), "qr", "M", nil, &lat, nil, 50)
	require.Error(t, err)
}

func TestNearby_RejectsBadLat(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	_, err := s.Nearby(context.Background(), 200, 0, 100, 5)
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestNearby_RejectsZeroRadius(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	_, err := s.Nearby(context.Background(), 0, 0, 0, 5)
	require.Error(t, err)
}

func TestNearby_RejectsRadiusTooLarge(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	_, err := s.Nearby(context.Background(), 0, 0, 99999, 5)
	require.Error(t, err)
}

func TestNearby_HappyPath(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	_, err := s.Nearby(context.Background(), 50.0, 19.0, 500, 0)
	require.NoError(t, err)
}

func TestUpdate_RejectsBadBattery(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	bad := -1
	_, err := s.Update(context.Background(), uuid.New(), scooters.UpdatePatch{BatteryLevel: &bad})
	require.Error(t, err)
}

func TestUpdate_RejectsBadStatus(t *testing.T) {
	s := scooters.New(&fakeRepo{})
	bad := "weird"
	_, err := s.Update(context.Background(), uuid.New(), scooters.UpdatePatch{Status: &bad})
	require.Error(t, err)
}
