package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	MaintOpen   = "open"
	MaintClosed = "closed"
)

type MaintenanceLog struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	ScooterID    uuid.UUID  `db:"scooter_id" json:"scooter_id"`
	Description  string     `db:"description" json:"description"`
	OpenedAt     time.Time  `db:"opened_at" json:"opened_at"`
	ClosedAt     *time.Time `db:"closed_at" json:"closed_at,omitempty"`
	TechnicianID *uuid.UUID `db:"technician_id" json:"technician_id,omitempty"`
	Status       string     `db:"status" json:"status"`
}
