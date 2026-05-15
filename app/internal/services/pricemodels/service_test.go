package pricemodels_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	pmsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/pricemodels"
	pmrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/pricemodels"
)

type fakeRepo struct {
	created *models.PriceModel
}

func (f *fakeRepo) Create(_ context.Context, pm *models.PriceModel) error {
	pm.ID = uuid.New()
	f.created = pm
	return nil
}
func (f *fakeRepo) Get(_ context.Context, _ uuid.UUID) (*models.PriceModel, error) {
	return f.created, nil
}
func (f *fakeRepo) List(_ context.Context, _ models.Page) ([]models.PriceModel, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Update(_ context.Context, _ uuid.UUID, _ pmrepo.UpdatePatch) (*models.PriceModel, error) {
	return f.created, nil
}
func (f *fakeRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func TestCreate_RejectsBlankName(t *testing.T) {
	s := pmsvc.New(&fakeRepo{})
	_, err := s.Create(context.Background(), pmsvc.CreateInput{Name: " "})
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestCreate_RejectsNegativeRates(t *testing.T) {
	s := pmsvc.New(&fakeRepo{})
	_, err := s.Create(context.Background(), pmsvc.CreateInput{
		Name:           "x",
		PricePerMinute: decimal.NewFromInt(-1),
	})
	require.Error(t, err)
}

func TestCreate_DefaultsCurrency(t *testing.T) {
	repo := &fakeRepo{}
	s := pmsvc.New(repo)
	pm, err := s.Create(context.Background(), pmsvc.CreateInput{
		Name:           "x",
		PricePerMinute: decimal.NewFromFloat(0.5),
		UnlockFee:      decimal.NewFromInt(1),
	})
	require.NoError(t, err)
	assert.Equal(t, "USD", pm.Currency)
}

func TestCreate_RejectsBadCurrency(t *testing.T) {
	s := pmsvc.New(&fakeRepo{})
	_, err := s.Create(context.Background(), pmsvc.CreateInput{
		Name:           "x",
		PricePerMinute: decimal.NewFromFloat(0.5),
		Currency:       "DOLLAR",
	})
	require.Error(t, err)
}

func TestUpdate_RejectsBadCurrency(t *testing.T) {
	s := pmsvc.New(&fakeRepo{})
	bad := "EU"
	_, err := s.Update(context.Background(), uuid.New(), pmsvc.UpdatePatch{Currency: &bad})
	require.Error(t, err)
}
