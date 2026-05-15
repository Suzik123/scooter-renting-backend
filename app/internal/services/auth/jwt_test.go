package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
)

func testCfg(ttl time.Duration) *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
			Issuer: "uniscoot-test",
			TTL:    ttl,
		},
		Bcrypt: config.BcryptConfig{Cost: 4},
	}
}

func newSvc(t *testing.T, cfg *config.Config) *authsvc.Service {
	t.Helper()
	s := authsvc.New(cfg, nil)
	return s
}

func TestIssueAndParseJWT_RoundTrip(t *testing.T) {
	s := newSvc(t, testCfg(time.Hour))
	uid := uuid.New()
	u := &models.User{ID: uid, Role: models.RoleUser}

	tok, err := s.IssueJWT(u)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	claims, err := s.ParseJWT(tok)
	require.NoError(t, err)
	assert.Equal(t, uid, claims.UserID)
	assert.Equal(t, models.RoleUser, claims.Role)
	assert.NotEmpty(t, claims.JTI(), "jti must be set")
	assert.False(t, claims.Expiry().IsZero(), "expiry must be set")
}

func TestParseJWT_RejectsBlankToken(t *testing.T) {
	s := newSvc(t, testCfg(time.Hour))
	_, err := s.ParseJWT("")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindUnauthorized))
}

func TestParseJWT_RejectsTamperedSignature(t *testing.T) {
	s := newSvc(t, testCfg(time.Hour))
	u := &models.User{ID: uuid.New(), Role: models.RoleUser}
	tok, err := s.IssueJWT(u)
	require.NoError(t, err)

	// Flip a byte in the signature segment.
	parts := strings.Split(tok, ".")
	require.Len(t, parts, 3)
	bad := parts[0] + "." + parts[1] + ".AAAA" + parts[2]
	_, err = s.ParseJWT(bad)
	require.Error(t, err)
}

func TestParseJWT_RejectsExpired(t *testing.T) {
	s := newSvc(t, testCfg(time.Hour))
	// Hand-craft an expired token.
	claims := authsvc.Claims{
		UserID: uuid.New(),
		Role:   models.RoleUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "uniscoot-test",
			Subject:   uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			ID:        uuid.NewString(),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	_, err = s.ParseJWT(raw)
	require.Error(t, err)
}

func TestParseJWT_RejectsWrongIssuer(t *testing.T) {
	s := newSvc(t, testCfg(time.Hour))
	claims := authsvc.Claims{
		UserID: uuid.New(),
		Role:   models.RoleUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "other-issuer",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			ID:        uuid.NewString(),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	_, err = s.ParseJWT(raw)
	require.Error(t, err)
}
