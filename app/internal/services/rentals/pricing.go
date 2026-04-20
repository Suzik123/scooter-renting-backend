package rentals

import (
	"math"
	"time"

	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

// CalculateCost computes the total rental cost for the given duration using the price model.
// Minutes are rounded up. If the price model has a daily cap, the result is clamped to it.
func CalculateCost(pm *models.PriceModel, duration time.Duration) decimal.Decimal {
	if pm == nil {
		return decimal.Zero
	}
	if duration < 0 {
		duration = 0
	}
	minutes := int64(math.Ceil(duration.Minutes()))
	if minutes < 0 {
		minutes = 0
	}
	cost := pm.UnlockFee.Add(pm.PerMinuteRate.Mul(decimal.NewFromInt(minutes)))
	if pm.DailyCap != nil && cost.GreaterThan(*pm.DailyCap) {
		cost = *pm.DailyCap
	}
	// Round to 2 decimal places; domain uses numeric(10,2).
	return cost.Round(2)
}
