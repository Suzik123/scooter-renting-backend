package rentals

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

// fakeZonesRepo returns a fixed slice of zones; List ignores the pagination
// args (the production code passes Limit: 1000, Offset: 0).
type fakeZonesRepo struct {
	zones []models.Zone
	calls int
}

func (f *fakeZonesRepo) List(_ context.Context, _ models.Page) ([]models.Zone, int, error) {
	f.calls++
	return f.zones, len(f.zones), nil
}

// newServiceForZoneTests builds a *Service with only the fields touched by
// enforceNoParkZone wired. Everything else is left nil — tests must only
// exercise the geo-check path.
func newServiceForZoneTests(z ZonesRepo) *Service {
	s := &Service{log: zap.NewNop()}
	s.SetZonesRepo(z)
	return s
}

func decPtr(f float64) *decimal.Decimal {
	d := decimal.NewFromFloat(f)
	return &d
}

func warsawNoPark() models.Zone {
	return models.Zone{
		ID:           uuid.New(),
		Name:         "Plac Defilad no_park",
		CenterLat:    decimal.NewFromFloat(52.2297),
		CenterLon:    decimal.NewFromFloat(21.0122),
		RadiusMeters: 100,
		ZoneType:     models.ZoneTypeNoPark,
	}
}

func TestEnd_BlocksInsideNoParkZone(t *testing.T) {
	z := warsawNoPark()
	fake := &fakeZonesRepo{zones: []models.Zone{z}}
	s := newServiceForZoneTests(fake)

	// Endpoint is the zone's center -> clearly inside.
	err := s.enforceNoParkZone(context.Background(), decPtr(52.2297), decPtr(21.0122))
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindZoneViolation))
	assert.Contains(t, err.Error(), "cannot_end_in_no_park_zone")
	assert.Equal(t, 1, fake.calls)
}

func TestEnd_AllowsOutsideNoParkZone(t *testing.T) {
	z := warsawNoPark()
	fake := &fakeZonesRepo{zones: []models.Zone{z}}
	s := newServiceForZoneTests(fake)

	// Far away (Krakow vs Warsaw, ~250 km).
	err := s.enforceNoParkZone(context.Background(), decPtr(50.0647), decPtr(19.9450))
	require.NoError(t, err)
}

func TestEnd_AllowsWhenNoCoords(t *testing.T) {
	fake := &fakeZonesRepo{zones: []models.Zone{warsawNoPark()}}
	s := newServiceForZoneTests(fake)

	require.NoError(t, s.enforceNoParkZone(context.Background(), nil, nil))
	require.NoError(t, s.enforceNoParkZone(context.Background(), decPtr(52.2297), nil))
	require.NoError(t, s.enforceNoParkZone(context.Background(), nil, decPtr(21.0122)))
	assert.Equal(t, 0, fake.calls, "zones repo must not be called when coords are missing")
}

func TestEnd_IgnoresServiceAndReducedSpeedZones(t *testing.T) {
	serviceZone := warsawNoPark()
	serviceZone.ZoneType = models.ZoneTypeService
	reduced := warsawNoPark()
	reduced.ZoneType = models.ZoneTypeReducedSpeed
	fake := &fakeZonesRepo{zones: []models.Zone{serviceZone, reduced}}
	s := newServiceForZoneTests(fake)

	// Coords are the center of those zones — would block if zone_type matched.
	err := s.enforceNoParkZone(context.Background(), decPtr(52.2297), decPtr(21.0122))
	require.NoError(t, err)
}

func TestEnd_NoEnforcementWhenZonesRepoUnwired(t *testing.T) {
	s := &Service{log: zap.NewNop()}
	// zones unset — must not panic and must allow.
	err := s.enforceNoParkZone(context.Background(), decPtr(52.2297), decPtr(21.0122))
	require.NoError(t, err)
}
