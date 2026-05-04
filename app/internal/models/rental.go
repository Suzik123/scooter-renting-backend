package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	RentalActive    = "active"
	RentalCompleted = "completed"
	RentalCancelled = "cancelled"
)

type Rental struct {
	ID           uuid.UUID        `db:"rental_id" json:"rental_id"`
	UserID       uuid.UUID        `db:"user_id" json:"user_id"`
	ScooterID    uuid.UUID        `db:"scooter_id" json:"scooter_id"`
	PriceModelID uuid.UUID        `db:"price_model_id" json:"price_model_id"`
	StartTime    time.Time        `db:"start_time" json:"start_time"`
	EndTime      *time.Time       `db:"end_time" json:"end_time,omitempty"`
	StartLat     *decimal.Decimal `db:"start_lat" json:"start_lat,omitempty"`
	StartLon     *decimal.Decimal `db:"start_lon" json:"start_lon,omitempty"`
	EndLat       *decimal.Decimal `db:"end_lat" json:"end_lat,omitempty"`
	EndLon       *decimal.Decimal `db:"end_lon" json:"end_lon,omitempty"`
	TotalCost    decimal.Decimal  `db:"total_cost" json:"total_cost"`
	Status       string           `db:"status" json:"status"`
	DistanceM    int              `db:"distance_m" json:"distance_m"`
	CreatedAt    time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at" json:"updated_at"`
}
