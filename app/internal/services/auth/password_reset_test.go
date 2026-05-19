package auth_test

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/hasher"

	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
)

// fakePRTokenRepo is an in-memory PasswordResetTokenRepo. Tokens are keyed
// by their hex hash (turned into a string) for simple lookups.
type fakePRTokenRepo struct {
	mu             sync.Mutex
	byHash         map[string]*authsvc.PasswordResetTokenRow
	byID           map[uuid.UUID]*authsvc.PasswordResetTokenRow
	invalidateCnt  int
}

func newFakePRTokenRepo() *fakePRTokenRepo {
	return &fakePRTokenRepo{
		byHash: map[string]*authsvc.PasswordResetTokenRow{},
		byID:   map[uuid.UUID]*authsvc.PasswordResetTokenRow{},
	}
}

func (f *fakePRTokenRepo) Create(_ context.Context, userID uuid.UUID, hash []byte, expiresAt time.Time) (*authsvc.PasswordResetTokenRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := &authsvc.PasswordResetTokenRow{
		TokenID:   uuid.New(),
		UserID:    userID,
		TokenHash: append([]byte(nil), hash...),
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}
	f.byHash[string(hash)] = row
	f.byID[row.TokenID] = row
	return row, nil
}

func (f *fakePRTokenRepo) GetByHash(_ context.Context, hash []byte) (*authsvc.PasswordResetTokenRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row, ok := f.byHash[string(hash)]; ok {
		cp := *row
		return &cp, nil
	}
	return nil, apperrors.NotFound("password reset token")
}

func (f *fakePRTokenRepo) InvalidateAllForUser(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidateCnt++
	now := time.Now().UTC()
	for _, row := range f.byHash {
		if row.UserID == userID && row.UsedAt == nil {
			row.UsedAt = &now
		}
	}
	return nil
}

func (f *fakePRTokenRepo) MarkUsed(_ context.Context, tokenID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.byID[tokenID]
	if !ok {
		return apperrors.NotFound("password reset token")
	}
	if row.UsedAt != nil {
		return apperrors.NotFound("password reset token")
	}
	now := time.Now().UTC()
	row.UsedAt = &now
	return nil
}

// fakePRUserRepo is the narrow PasswordResetUserRepo. It records the most
// recent hash so tests can verify password rotation.
type fakePRUserRepo struct {
	mu      sync.Mutex
	hashes  map[uuid.UUID]string
	resetCnt int
}

func newFakePRUserRepo() *fakePRUserRepo {
	return &fakePRUserRepo{hashes: map[uuid.UUID]string{}}
}

func (f *fakePRUserRepo) ResetPassword(_ context.Context, id uuid.UUID, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hashes[id] = hash
	f.resetCnt++
	return nil
}

// fakePublisher captures every published envelope.
type fakePublisher struct {
	mu    sync.Mutex
	calls []events.Envelope
}

func (p *fakePublisher) Publish(_ context.Context, e events.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, e)
	return nil
}

func newPasswordResetSvc(t *testing.T, prTokens authsvc.PasswordResetTokenRepo, prUsers authsvc.PasswordResetUserRepo, pub events.Publisher) (*authsvc.Service, *fakeUserRepo) {
	t.Helper()
	users := newFakeUserRepo()
	s := authsvc.New(testCfg(time.Hour), users)
	s.SetPasswordResetDeps(prTokens, prUsers, pub, zap.NewNop(), time.Minute*30, "http://localhost:5173")
	return s, users
}

func TestRequestPasswordReset_PublishesEvent(t *testing.T) {
	tokens := newFakePRTokenRepo()
	pub := &fakePublisher{}
	s, users := newPasswordResetSvc(t, tokens, newFakePRUserRepo(), pub)

	_, _, err := s.Register(context.Background(), "a@example.com", "Alice", "", "supersecret", "")
	require.NoError(t, err)

	require.NoError(t, s.RequestPasswordReset(context.Background(), "A@Example.com"))
	require.Len(t, pub.calls, 1)
	assert.Equal(t, events.TypePasswordResetRequested, pub.calls[0].Type)

	// One token row inserted for this user.
	assert.Len(t, tokens.byHash, 1)
	for _, row := range tokens.byHash {
		assert.Equal(t, users.byEmail["a@example.com"].ID, row.UserID)
		assert.Nil(t, row.UsedAt)
	}
}

func TestRequestPasswordReset_SilentOnUnknownEmail(t *testing.T) {
	tokens := newFakePRTokenRepo()
	pub := &fakePublisher{}
	s, _ := newPasswordResetSvc(t, tokens, newFakePRUserRepo(), pub)

	require.NoError(t, s.RequestPasswordReset(context.Background(), "nobody@example.com"))
	assert.Empty(t, pub.calls)
	assert.Empty(t, tokens.byHash)
}

func TestRequestPasswordReset_InvalidatesPriorTokens(t *testing.T) {
	tokens := newFakePRTokenRepo()
	pub := &fakePublisher{}
	s, users := newPasswordResetSvc(t, tokens, newFakePRUserRepo(), pub)

	_, _, err := s.Register(context.Background(), "b@example.com", "Bob", "", "supersecret", "")
	require.NoError(t, err)
	uid := users.byEmail["b@example.com"].ID

	require.NoError(t, s.RequestPasswordReset(context.Background(), "b@example.com"))
	require.NoError(t, s.RequestPasswordReset(context.Background(), "b@example.com"))

	// Two creates, but the first row must be marked used by the second
	// Request's pre-invalidation.
	var active int
	for _, row := range tokens.byHash {
		if row.UserID == uid && row.UsedAt == nil {
			active++
		}
	}
	assert.Equal(t, 1, active)
	assert.GreaterOrEqual(t, tokens.invalidateCnt, 2)
}

func TestRequestPasswordReset_RejectsBlankEmail(t *testing.T) {
	tokens := newFakePRTokenRepo()
	s, _ := newPasswordResetSvc(t, tokens, newFakePRUserRepo(), &fakePublisher{})
	err := s.RequestPasswordReset(context.Background(), "  ")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestConfirmPasswordReset_HappyPath(t *testing.T) {
	tokens := newFakePRTokenRepo()
	prUsers := newFakePRUserRepo()
	pub := &fakePublisher{}
	s, users := newPasswordResetSvc(t, tokens, prUsers, pub)

	_, _, err := s.Register(context.Background(), "c@example.com", "Carol", "", "originalpass", "")
	require.NoError(t, err)
	uid := users.byEmail["c@example.com"].ID

	require.NoError(t, s.RequestPasswordReset(context.Background(), "c@example.com"))
	rawToken := extractTokenFromURL(t, pub.calls[0])

	require.NoError(t, s.ConfirmPasswordReset(context.Background(), rawToken, "brandnewpass"))

	// New hash persisted + verifiable.
	newHash, ok := prUsers.hashes[uid]
	require.True(t, ok)
	require.NoError(t, hasher.Compare(newHash, "brandnewpass"))

	// Token row marked used.
	for _, row := range tokens.byHash {
		if row.UserID == uid {
			require.NotNil(t, row.UsedAt)
		}
	}
}

func TestConfirmPasswordReset_RejectsExpired(t *testing.T) {
	tokens := newFakePRTokenRepo()
	prUsers := newFakePRUserRepo()
	pub := &fakePublisher{}
	users := newFakeUserRepo()
	s := authsvc.New(testCfg(time.Hour), users)
	// 1 ns TTL so the inserted token is already expired by the time
	// Confirm runs.
	s.SetPasswordResetDeps(tokens, prUsers, pub, zap.NewNop(), time.Nanosecond, "http://localhost:5173")

	_, _, err := s.Register(context.Background(), "d@example.com", "D", "", "originalpass", "")
	require.NoError(t, err)
	require.NoError(t, s.RequestPasswordReset(context.Background(), "d@example.com"))
	rawToken := extractTokenFromURL(t, pub.calls[0])
	time.Sleep(2 * time.Millisecond)

	err = s.ConfirmPasswordReset(context.Background(), rawToken, "brandnewpass")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
	assert.Contains(t, err.Error(), "invalid_token")
}

func TestConfirmPasswordReset_RejectsAlreadyUsed(t *testing.T) {
	tokens := newFakePRTokenRepo()
	prUsers := newFakePRUserRepo()
	pub := &fakePublisher{}
	s, _ := newPasswordResetSvc(t, tokens, prUsers, pub)

	_, _, err := s.Register(context.Background(), "e@example.com", "E", "", "originalpass", "")
	require.NoError(t, err)
	require.NoError(t, s.RequestPasswordReset(context.Background(), "e@example.com"))
	rawToken := extractTokenFromURL(t, pub.calls[0])

	require.NoError(t, s.ConfirmPasswordReset(context.Background(), rawToken, "brandnewpass"))
	err = s.ConfirmPasswordReset(context.Background(), rawToken, "anothernewpass")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

func TestConfirmPasswordReset_RejectsShortPassword(t *testing.T) {
	tokens := newFakePRTokenRepo()
	prUsers := newFakePRUserRepo()
	pub := &fakePublisher{}
	s, _ := newPasswordResetSvc(t, tokens, prUsers, pub)

	_, _, err := s.Register(context.Background(), "f@example.com", "F", "", "originalpass", "")
	require.NoError(t, err)
	require.NoError(t, s.RequestPasswordReset(context.Background(), "f@example.com"))
	rawToken := extractTokenFromURL(t, pub.calls[0])

	err = s.ConfirmPasswordReset(context.Background(), rawToken, "short")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestConfirmPasswordReset_RejectsUnknownToken(t *testing.T) {
	tokens := newFakePRTokenRepo()
	s, _ := newPasswordResetSvc(t, tokens, newFakePRUserRepo(), &fakePublisher{})
	err := s.ConfirmPasswordReset(context.Background(), "deadbeef", "supersecretpassword")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

// extractTokenFromURL pulls the ?token=... value out of the published
// PasswordResetRequested envelope. Used to mirror what the frontend would
// see when the user clicks the email link.
func extractTokenFromURL(t *testing.T, env events.Envelope) string {
	t.Helper()
	require.Equal(t, events.TypePasswordResetRequested, env.Type)
	var p events.PasswordResetRequested
	require.NoError(t, json.Unmarshal(env.Payload, &p))
	u, err := url.Parse(p.ResetURL)
	require.NoError(t, err)
	tok := u.Query().Get("token")
	require.NotEmpty(t, tok)
	return tok
}
