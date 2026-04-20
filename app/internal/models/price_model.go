package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PriceModel struct {
	ID            uuid.UUID        `db:"id" json:"id"`
	Name          string           `db:"name" json:"name"`
	PerMinuteRate decimal.Decimal  `db:"per_minute_rate" json:"per_minute_rate"`
	UnlockFee     decimal.Decimal  `db:"unlock_fee" json:"unlock_fee"`
	DailyCap      *decimal.Decimal `db:"daily_cap" json:"daily_cap,omitempty"`
	Currency      string           `db:"currency" json:"currency"`
	CreatedAt     time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time        `db:"updated_at" json:"updated_at"`
}
