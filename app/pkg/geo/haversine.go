package geo

import (
	"math"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

const earthRadiusM = 6371000.0

// DistanceMeters returns great-circle distance between two lat/lng points in meters.
func DistanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// IsInsideZone reports whether the given lat/lon falls within the circular
// area defined by z (center + radius). Boundary is treated as inside.
func IsInsideZone(lat, lon float64, z models.Zone) bool {
	centerLat, _ := z.CenterLat.Float64()
	centerLon, _ := z.CenterLon.Float64()
	return DistanceMeters(lat, lon, centerLat, centerLon) <= float64(z.RadiusMeters)
}
