package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	ZoneTypeService       = "service"
	ZoneTypeNoPark        = "no_park"
	ZoneTypeReducedSpeed  = "reduced_speed"
)

type Zone struct {
	ID            uuid.UUID       `db:"zone_id" json:"zone_id"`
	Name          string          `db:"name" json:"name"`
	CenterLat     decimal.Decimal `db:"center_lat" json:"center_lat"`
	CenterLon     decimal.Decimal `db:"center_lon" json:"center_lon"`
	RadiusMeters  int             `db:"radius_meters" json:"radius_meters"`
	ZoneType      string          `db:"zone_type" json:"zone_type"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}
