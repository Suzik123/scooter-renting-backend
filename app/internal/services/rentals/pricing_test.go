package rentals_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/services/rentals"
)

func mkPM(perMin, unlock string, dailyCap *string) *models.PriceModel {
	pm := &models.PriceModel{
		UnlockFee:      decimalOf(unlock),
		PricePerMinute: decimalOf(perMin),
	}
	if dailyCap != nil {
		d := decimalOf(*dailyCap)
		pm.DailyCap = &d
	}
	return pm
}

func decimalOf(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func TestCalculateCost_ShortRideRoundsUpMinutes(t *testing.T) {
	pm := mkPM("0.50", "1.00", nil)
	// 90 seconds → ceil = 2 minutes → 1 + 2*0.5 = 2.00.
	cost := rentals.CalculateCost(pm, 90*time.Second)
	assert.Equal(t, "2", cost.String())
}

func TestCalculateCost_DailyCapClamps(t *testing.T) {
	cap_ := "5.00"
	pm := mkPM("1.00", "0.00", &cap_)
	cost := rentals.CalculateCost(pm, 24*time.Hour)
	assert.Equal(t, "5", cost.String())
}

func TestCalculateCost_ZeroDuration(t *testing.T) {
	pm := mkPM("0.50", "1.00", nil)
	cost := rentals.CalculateCost(pm, 0)
	assert.Equal(t, "1", cost.String())
}

func TestCalculateCost_NegativeDurationClampsToZero(t *testing.T) {
	pm := mkPM("0.50", "0.00", nil)
	cost := rentals.CalculateCost(pm, -5*time.Minute)
	assert.Equal(t, "0", cost.String())
}

func TestCalculateCost_NilPriceModel(t *testing.T) {
	assert.True(t, rentals.CalculateCost(nil, time.Minute).IsZero())
}
