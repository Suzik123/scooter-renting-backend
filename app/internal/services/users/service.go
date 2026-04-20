package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	usersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/users"
)

// Repo is the subset of the users repository consumed by this service.
type Repo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, patch usersrepo.UpdatePatch) (*models.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	GetWallet(ctx context.Context, id uuid.UUID) (decimal.Decimal, error)
	AdjustWallet(ctx context.Context, id uuid.UUID, delta decimal.Decimal) (decimal.Decimal, error)
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

type UpdatePatch struct {
	Name  *string
	Phone *string
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.User, error) {
	return s.repo.Update(ctx, id, usersrepo.UpdatePatch{Name: patch.Name, Phone: patch.Phone})
}

func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, id)
}

func (s *Service) GetWallet(ctx context.Context, id uuid.UUID) (decimal.Decimal, error) {
	return s.repo.GetWallet(ctx, id)
}

// TopUp adds a positive amount to the user's wallet and returns the resulting balance.
func (s *Service) TopUp(ctx context.Context, id uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	if amount.Sign() <= 0 {
		return decimal.Zero, apperrors.Invalid("amount must be positive")
	}
	return s.repo.AdjustWallet(ctx, id, amount)
}
