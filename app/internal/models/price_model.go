package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PriceModel struct {
	ID             uuid.UUID        `db:"price_model_id" json:"price_model_id"`
	Name           string           `db:"name" json:"name"`
	UnlockFee      decimal.Decimal  `db:"unlock_fee" json:"unlock_fee"`
	PricePerMinute decimal.Decimal  `db:"price_per_minute" json:"price_per_minute"`
	Currency       string           `db:"currency" json:"currency"`
	DailyCap       *decimal.Decimal `db:"daily_cap" json:"daily_cap,omitempty"`
	CreatedAt      time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time        `db:"updated_at" json:"updated_at"`
}
