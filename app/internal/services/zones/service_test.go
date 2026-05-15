package zones_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/services/zones"
	zonesrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/zones"
)

type fakeRepo struct {
	createErr error
	created   *models.Zone
	deleteErr error
}

func (f *fakeRepo) Create(_ context.Context, z *models.Zone) error {
	if f.createErr != nil {
		return f.createErr
	}
	z.ID = uuid.New()
	f.created = z
	return nil
}
func (f *fakeRepo) Get(_ context.Context, _ uuid.UUID) (*models.Zone, error) {
	return f.created, nil
}
func (f *fakeRepo) List(_ context.Context, _ models.Page) ([]models.Zone, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Update(_ context.Context, _ uuid.UUID, _ zonesrepo.UpdatePatch) (*models.Zone, error) {
	return f.created, nil
}
func (f *fakeRepo) Delete(_ context.Context, _ uuid.UUID) error { return f.deleteErr }

func TestCreate_RejectsBlankName(t *testing.T) {
	s := zones.New(&fakeRepo{})
	_, err := s.Create(context.Background(), zones.CreateInput{Name: "  ", CenterLat: decimal.Zero, CenterLon: decimal.Zero, RadiusMeters: 100})
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestCreate_RejectsNonPositiveRadius(t *testing.T) {
	s := zones.New(&fakeRepo{})
	_, err := s.Create(context.Background(), zones.CreateInput{Name: "x", RadiusMeters: 0})
	require.Error(t, err)
}

func TestCreate_RejectsBadZoneType(t *testing.T) {
	s := zones.New(&fakeRepo{})
	_, err := s.Create(context.Background(), zones.CreateInput{Name: "x", RadiusMeters: 1, ZoneType: "weird"})
	require.Error(t, err)
}

func TestCreate_DefaultsZoneTypeToService(t *testing.T) {
	repo := &fakeRepo{}
	s := zones.New(repo)
	z, err := s.Create(context.Background(), zones.CreateInput{Name: "Zone", RadiusMeters: 100})
	require.NoError(t, err)
	assert.Equal(t, models.ZoneTypeService, z.ZoneType)
}

func TestCreate_PropagatesRepoError(t *testing.T) {
	s := zones.New(&fakeRepo{createErr: errors.New("db down")})
	_, err := s.Create(context.Background(), zones.CreateInput{Name: "x", RadiusMeters: 1})
	require.Error(t, err)
}

func TestUpdate_RejectsBadZoneType(t *testing.T) {
	s := zones.New(&fakeRepo{})
	bad := "nope"
	_, err := s.Update(context.Background(), uuid.New(), zones.UpdatePatch{ZoneType: &bad})
	require.Error(t, err)
}

func TestUpdate_RejectsBadRadius(t *testing.T) {
	s := zones.New(&fakeRepo{})
	r := 0
	_, err := s.Update(context.Background(), uuid.New(), zones.UpdatePatch{RadiusMeters: &r})
	require.Error(t, err)
}

func TestDelete_PassesThrough(t *testing.T) {
	s := zones.New(&fakeRepo{})
	assert.NoError(t, s.Delete(context.Background(), uuid.New()))
}
