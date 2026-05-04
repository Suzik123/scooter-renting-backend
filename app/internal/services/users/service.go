package users

import (
	"context"

	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	usersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/users"
)

// Repo is the subset of the users repository consumed by this service.
type Repo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, patch usersrepo.UpdatePatch) (*models.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repo
}

func New(repo Repo) *Service {
	return &Service{repo: repo}
}

type UpdatePatch struct {
	FirstName   *string
	LastName    *string
	PhoneNumber *string
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, patch UpdatePatch) (*models.User, error) {
	return s.repo.Update(ctx, id, usersrepo.UpdatePatch{
		FirstName:   patch.FirstName,
		LastName:    patch.LastName,
		PhoneNumber: patch.PhoneNumber,
	})
}

func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, id)
}
