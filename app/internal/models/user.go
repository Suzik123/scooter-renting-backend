package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

const (
	UserActive    = "active"
	UserSuspended = "suspended"
	UserDeleted   = "deleted"
)

type User struct {
	ID               uuid.UUID `db:"user_id" json:"user_id"`
	FirstName        string    `db:"first_name" json:"first_name"`
	LastName         string    `db:"last_name" json:"last_name"`
	Email            string    `db:"email" json:"email"`
	PhoneNumber      *string   `db:"phone_number" json:"phone_number,omitempty"`
	RegistrationDate time.Time `db:"registration_date" json:"registration_date"`
	Status           string    `db:"status" json:"status"`
	Role             string    `db:"role" json:"role"`
	PasswordHash     *string   `db:"password_hash" json:"-"`
	OAuthProvider    *string   `db:"oauth_provider" json:"oauth_provider,omitempty"`
	OAuthSubject     *string   `db:"oauth_subject" json:"-"`
	StripeCustomerID *string   `db:"stripe_customer_id" json:"-"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}
