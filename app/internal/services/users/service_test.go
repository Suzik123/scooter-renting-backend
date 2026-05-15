package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/services/users"
	usersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/users"
)

type fakeRepo struct {
	got *models.User
}

func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	return &models.User{ID: id, Email: "x@x.com"}, nil
}
func (f *fakeRepo) Update(_ context.Context, id uuid.UUID, _ usersrepo.UpdatePatch) (*models.User, error) {
	u := &models.User{ID: id}
	f.got = u
	return u, nil
}
func (f *fakeRepo) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }

func TestGet_PassesThrough(t *testing.T) {
	s := users.New(&fakeRepo{})
	u, err := s.Get(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "x@x.com", u.Email)
}

func TestUpdate_PassesThrough(t *testing.T) {
	repo := &fakeRepo{}
	s := users.New(repo)
	name := "Alice"
	_, err := s.Update(context.Background(), uuid.New(), users.UpdatePatch{FirstName: &name})
	require.NoError(t, err)
}

func TestSoftDelete_PassesThrough(t *testing.T) {
	s := users.New(&fakeRepo{})
	require.NoError(t, s.SoftDelete(context.Background(), uuid.New()))
}
