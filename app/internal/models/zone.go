package models

import (
	"time"

	"github.com/google/uuid"
)

type Zone struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Boundary  *string   `db:"boundary" json:"boundary,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
