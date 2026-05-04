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
	ScooterRetired     = "retired"
)

type Scooter struct {
	ID           uuid.UUID        `db:"scooter_id" json:"scooter_id"`
	QRCode       string           `db:"qr_code" json:"qr_code"`
	BatteryLevel int              `db:"battery_level" json:"battery_level"`
	Status       string           `db:"status" json:"status"`
	ZoneID       *uuid.UUID       `db:"zone_id" json:"zone_id,omitempty"`
	Model        string           `db:"model" json:"model"`
	Lat          *decimal.Decimal `db:"lat" json:"lat,omitempty"`
	Lng          *decimal.Decimal `db:"lng" json:"lng,omitempty"`
	CreatedAt    time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time       `db:"deleted_at" json:"deleted_at,omitempty"`
}
