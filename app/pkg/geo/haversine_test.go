package geo_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/geo"
)

// At 52 deg latitude one degree of longitude is roughly 68_000 m, so 0.0001
// deg of longitude is ~6.85 m. We use a 50 m radius and place test points
// well inside / on the boundary / outside to avoid floating-point ties.
func TestIsInsideZone(t *testing.T) {
	zone := models.Zone{
		CenterLat:    decimal.NewFromFloat(52.2297),
		CenterLon:    decimal.NewFromFloat(21.0122),
		RadiusMeters: 50,
		ZoneType:     models.ZoneTypeNoPark,
	}

	cases := []struct {
		name   string
		lat    float64
		lon    float64
		inside bool
	}{
		{"center", 52.2297, 21.0122, true},
		{"1 m inside", 52.22971, 21.0122, true},
		{"well inside", 52.22974, 21.01225, true},
		{"on boundary (~50 m east)", 52.2297, 21.012933, true},
		{"1 m outside", 52.23015, 21.0122, false},
		{"far away", 51.0, 17.0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := geo.IsInsideZone(tc.lat, tc.lon, zone)
			assert.Equal(t, tc.inside, got)
		})
	}
}

func TestDistanceMeters_Symmetric(t *testing.T) {
	a := geo.DistanceMeters(52.0, 21.0, 52.01, 21.01)
	b := geo.DistanceMeters(52.01, 21.01, 52.0, 21.0)
	assert.InDelta(t, a, b, 1e-6)
}
