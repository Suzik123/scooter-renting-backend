package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	Email         string          `db:"email" json:"email"`
	Name          string          `db:"name" json:"name"`
	Phone         *string         `db:"phone" json:"phone,omitempty"`
	PasswordHash  *string         `db:"password_hash" json:"-"`
	OAuthID       *string         `db:"oauth_id" json:"oauth_id,omitempty"`
	Role          string          `db:"role" json:"role"`
	WalletBalance decimal.Decimal `db:"wallet_balance" json:"wallet_balance"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time      `db:"deleted_at" json:"deleted_at,omitempty"`
}
