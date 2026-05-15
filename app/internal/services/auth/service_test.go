package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/hasher"

	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
)

// fakeUserRepo implements authsvc.UserRepo in-memory.
type fakeUserRepo struct {
	mu       sync.Mutex
	byEmail  map[string]*models.User
	byID     map[uuid.UUID]*models.User
	createErr error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byEmail: map[string]*models.User{},
		byID:    map[uuid.UUID]*models.User{},
	}
}

func (r *fakeUserRepo) Create(_ context.Context, u *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	if _, ok := r.byEmail[u.Email]; ok {
		return apperrors.Conflict("email already exists")
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	cp := *u
	r.byEmail[u.Email] = &cp
	r.byID[u.ID] = &cp
	return nil
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, apperrors.NotFound("user")
}

func (r *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, apperrors.NotFound("user")
}

// fakeBlacklist implements authsvc.Blacklist in-memory.
type fakeBlacklist struct {
	mu    sync.Mutex
	store map[string]time.Time
	err   error
}

func newFakeBlacklist() *fakeBlacklist {
	return &fakeBlacklist{store: map[string]time.Time{}}
}

func (b *fakeBlacklist) Revoke(_ context.Context, jti string, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.store[jti] = time.Now().Add(ttl)
	return nil
}

func (b *fakeBlacklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return false, b.err
	}
	exp, ok := b.store[jti]
	if !ok {
		return false, nil
	}
	return time.Now().Before(exp), nil
}

func TestRegister_Success(t *testing.T) {
	cfg := testCfg(time.Hour)
	repo := newFakeUserRepo()
	s := authsvc.New(cfg, repo)

	u, tok, err := s.Register(context.Background(), "Alice@Example.com", "Alice", "Doe", "supersecret", "")
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Equal(t, models.RoleUser, u.Role)
	assert.NotEmpty(t, tok)
	require.NotNil(t, u.PasswordHash)
	require.NoError(t, hasher.Compare(*u.PasswordHash, "supersecret"))
}

func TestRegister_RejectsEmptyEmail(t *testing.T) {
	repo := newFakeUserRepo()
	s := authsvc.New(testCfg(time.Hour), repo)
	_, _, err := s.Register(context.Background(), "", "n", "n", "12345678", "")
	require.Error(t, err)
}

func TestLogin_SuccessAndBadPassword(t *testing.T) {
	cfg := testCfg(time.Hour)
	repo := newFakeUserRepo()
	s := authsvc.New(cfg, repo)

	_, _, err := s.Register(context.Background(), "bob@example.com", "Bob", "", "pass1234", "")
	require.NoError(t, err)

	_, tok, err := s.Login(context.Background(), "bob@example.com", "pass1234")
	require.NoError(t, err)
	assert.NotEmpty(t, tok)

	_, _, err = s.Login(context.Background(), "bob@example.com", "wrong")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

func TestLogout_RevokesToken(t *testing.T) {
	cfg := testCfg(time.Hour)
	repo := newFakeUserRepo()
	s := authsvc.New(cfg, repo)
	bl := newFakeBlacklist()
	s.SetBlacklist(bl)

	_, tok, err := s.Register(context.Background(), "c@example.com", "C", "", "pass1234", "")
	require.NoError(t, err)

	// Pre-condition: token not revoked yet.
	claims, err := s.ParseJWT(tok)
	require.NoError(t, err)
	revoked, err := s.IsRevoked(context.Background(), claims.JTI())
	require.NoError(t, err)
	assert.False(t, revoked)

	// Logout.
	require.NoError(t, s.Logout(context.Background(), tok))

	revoked, err = s.IsRevoked(context.Background(), claims.JTI())
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestLogout_RejectsInvalidToken(t *testing.T) {
	cfg := testCfg(time.Hour)
	s := authsvc.New(cfg, newFakeUserRepo())
	s.SetBlacklist(newFakeBlacklist())

	err := s.Logout(context.Background(), "not-a-jwt")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

func TestLogout_InvalidTokenRejected(t *testing.T) {
	cfg := testCfg(time.Hour)
	s := authsvc.New(cfg, newFakeUserRepo())
	s.SetBlacklist(newFakeBlacklist())
	err := s.Logout(context.Background(), "garbage.token.value")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

func TestBlacklist_PropagatesError(t *testing.T) {
	cfg := testCfg(time.Hour)
	s := authsvc.New(cfg, newFakeUserRepo())
	bl := newFakeBlacklist()
	bl.err = errors.New("redis down")
	s.SetBlacklist(bl)
	revoked, err := s.IsRevoked(context.Background(), uuid.NewString())
	require.Error(t, err)
	assert.False(t, revoked)
}
