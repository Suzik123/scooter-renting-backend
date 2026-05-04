package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	MaintOpen   = "open"
	MaintClosed = "closed"
)

type MaintenanceLog struct {
	ID               uuid.UUID        `db:"maintenance_id" json:"maintenance_id"`
	ScooterID        uuid.UUID        `db:"scooter_id" json:"scooter_id"`
	TechnicianName   string           `db:"technician_name" json:"technician_name"`
	IssueDescription string           `db:"issue_description" json:"issue_description"`
	RepairCost       *decimal.Decimal `db:"repair_cost" json:"repair_cost,omitempty"`
	StartDate        time.Time        `db:"start_date" json:"start_date"`
	EndDate          *time.Time       `db:"end_date" json:"end_date,omitempty"`
	Status           string           `db:"status" json:"status"`
}
