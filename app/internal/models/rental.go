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
	ID           uuid.UUID       `db:"id" json:"id"`
	UserID       uuid.UUID       `db:"user_id" json:"user_id"`
	ScooterID    uuid.UUID       `db:"scooter_id" json:"scooter_id"`
	PriceModelID uuid.UUID       `db:"price_model_id" json:"price_model_id"`
	StartedAt    time.Time       `db:"started_at" json:"started_at"`
	EndedAt      *time.Time      `db:"ended_at" json:"ended_at,omitempty"`
	DistanceM    int             `db:"distance_m" json:"distance_m"`
	TotalCost    decimal.Decimal `db:"total_cost" json:"total_cost"`
	Status       string          `db:"status" json:"status"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}
