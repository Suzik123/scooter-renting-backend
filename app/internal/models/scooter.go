package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	ScooterAvailable   = "available"
	ScooterRented      = "rented"
	ScooterMaintenance = "maintenance"
)

type Scooter struct {
	ID         uuid.UUID        `db:"id" json:"id"`
	Code       string           `db:"code" json:"code"`
	Model      string           `db:"model" json:"model"`
	BatteryPct int              `db:"battery_pct" json:"battery_pct"`
	Status     string           `db:"status" json:"status"`
	ZoneID     *uuid.UUID       `db:"zone_id" json:"zone_id,omitempty"`
	Lat        *decimal.Decimal `db:"lat" json:"lat,omitempty"`
	Lng        *decimal.Decimal `db:"lng" json:"lng,omitempty"`
	CreatedAt  time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time        `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time       `db:"deleted_at" json:"deleted_at,omitempty"`
}
