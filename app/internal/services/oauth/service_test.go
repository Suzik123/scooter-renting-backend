package oauth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	googleclient "github.com/uniscoot/scooter-renting-backend/app/clients/google"
	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
	oauthsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/oauth"
)

type fakeUserRepo struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*models.User
	byEmail map[string]*models.User
	byOauth map[string]*models.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:    map[uuid.UUID]*models.User{},
		byEmail: map[string]*models.User{},
		byOauth: map[string]*models.User{},
	}
}

func keyFor(provider, subject string) string { return provider + "|" + subject }

func (r *fakeUserRepo) Create(_ context.Context, u *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byEmail[u.Email]; ok {
		return apperrors.Conflict("email already exists")
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	cp := *u
	r.byEmail[u.Email] = &cp
	r.byID[u.ID] = &cp
	if u.OAuthProvider != nil && u.OAuthSubject != nil {
		r.byOauth[keyFor(*u.OAuthProvider, *u.OAuthSubject)] = &cp
	}
	return nil
}
func (r *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, apperrors.NotFound("user")
}
func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, apperrors.NotFound("user")
}
func (r *fakeUserRepo) GetByOAuth(_ context.Context, provider, subject string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byOauth[keyFor(provider, subject)]; ok {
		return u, nil
	}
	return nil, apperrors.NotFound("user")
}
func (r *fakeUserRepo) LinkOAuth(_ context.Context, id uuid.UUID, provider, subject string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, apperrors.NotFound("user")
	}
	p := provider
	s := subject
	u.OAuthProvider = &p
	u.OAuthSubject = &s
	r.byOauth[keyFor(provider, subject)] = u
	return u, nil
}

type fakeGoogle struct {
	claims *googleclient.Claims
	err    error
}

func (g *fakeGoogle) VerifyIDToken(_ context.Context, _ string) (*googleclient.Claims, error) {
	return g.claims, g.err
}

func newSvc(repo *fakeUserRepo, gc *fakeGoogle, hostedDomain string) *oauthsvc.Service {
	cfg := &config.Config{
		JWT:    config.JWTConfig{Secret: "s", Issuer: "iss", TTL: time.Hour},
		Bcrypt: config.BcryptConfig{Cost: 4},
		Google: config.GoogleConfig{HostedDomain: hostedDomain},
	}
	auth := authsvc.New(cfg, repo)
	return oauthsvc.New(repo, auth, gc, cfg)
}

func TestVerifyAndLogin_RejectsInvalidToken(t *testing.T) {
	repo := newFakeUserRepo()
	gc := &fakeGoogle{err: errors.New("bad token")}
	s := newSvc(repo, gc, "")
	_, _, err := s.VerifyAndLogin(context.Background(), "x")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

func TestVerifyAndLogin_RejectsWrongHostedDomain(t *testing.T) {
	repo := newFakeUserRepo()
	gc := &fakeGoogle{claims: &googleclient.Claims{
		Subject: "sub1", Email: "a@gmail.com", EmailVerified: true, HostedDomain: "other.com",
	}}
	s := newSvc(repo, gc, "uniscoot.com")
	_, _, err := s.VerifyAndLogin(context.Background(), "x")
	require.Error(t, err)
}

func TestVerifyAndLogin_NewUserProvisioned(t *testing.T) {
	repo := newFakeUserRepo()
	gc := &fakeGoogle{claims: &googleclient.Claims{
		Subject: "sub1", Email: "new@x.com", EmailVerified: true, GivenName: "G", FamilyName: "F",
	}}
	s := newSvc(repo, gc, "")
	u, tok, err := s.VerifyAndLogin(context.Background(), "x")
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
	assert.Equal(t, "new@x.com", u.Email)
	assert.NotNil(t, u.OAuthSubject)
}

func TestVerifyAndLogin_ExistingByOAuth(t *testing.T) {
	repo := newFakeUserRepo()
	prov := "google"
	sub := "sub2"
	existing := &models.User{ID: uuid.New(), Email: "e@x.com", Role: models.RoleUser, OAuthProvider: &prov, OAuthSubject: &sub}
	require.NoError(t, repo.Create(context.Background(), existing))
	gc := &fakeGoogle{claims: &googleclient.Claims{
		Subject: "sub2", Email: "e@x.com", EmailVerified: true,
	}}
	s := newSvc(repo, gc, "")
	u, tok, err := s.VerifyAndLogin(context.Background(), "x")
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
	assert.Equal(t, existing.ID, u.ID)
}
