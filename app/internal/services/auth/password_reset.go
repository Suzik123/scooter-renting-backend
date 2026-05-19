package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/hasher"
)

// tokenByteLen is the length of the raw token before hex-encoding. 32 bytes
// gives 256 bits of entropy and produces a 64-character hex string.
const tokenByteLen = 32

// minPasswordLen mirrors the registration policy (handler-level validation).
const minPasswordLen = 8

// unknownEmailSleep is the fixed delay used on the unknown-email branch of
// RequestPasswordReset to flatten the timing oracle (so attackers can't
// distinguish "no such email" from "email sent").
const unknownEmailSleep = 200 * time.Millisecond

// PasswordResetTokenRepo persists the bytea token rows. Kept narrow on
// purpose so the auth.Service is easy to test with a fake.
type PasswordResetTokenRepo interface {
	Create(ctx context.Context, userID uuid.UUID, hash []byte, expiresAt time.Time) (*PasswordResetTokenRow, error)
	GetByHash(ctx context.Context, hash []byte) (*PasswordResetTokenRow, error)
	InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error
	MarkUsed(ctx context.Context, tokenID uuid.UUID) error
}

// PasswordResetTokenRow is the domain-level shape this package consumes. It
// intentionally mirrors the repo's Token type so the binding adapter is a
// one-line copy.
type PasswordResetTokenRow struct {
	TokenID   uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// PasswordResetUserRepo widens UserRepo with the password mutator. We add it
// as a separate interface so the existing Register/Login fake doesn't have
// to implement it.
type PasswordResetUserRepo interface {
	ResetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

// SetPasswordResetDeps wires the deps needed for the reset flow. Kept
// separate from New() so the existing service constructor stays untouched.
// Any nil dep disables the password reset path (returns Internal).
func (s *Service) SetPasswordResetDeps(repo PasswordResetTokenRepo, users PasswordResetUserRepo, pub events.Publisher, log *zap.Logger, ttl time.Duration, frontendBaseURL string) {
	s.prTokens = repo
	s.prUsers = users
	s.prPub = pub
	if log == nil {
		log = zap.NewNop()
	}
	s.prLog = log
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	s.prTTL = ttl
	s.prFrontendBaseURL = strings.TrimRight(frontendBaseURL, "/")
}

// RequestPasswordReset issues a fresh reset token for the user matching
// email and publishes a PasswordResetRequested event. Unknown emails are
// silently dropped (after a small fixed sleep) so the endpoint cannot be
// used to enumerate accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return apperrors.Invalid("email is required")
	}
	if !looksLikeEmail(email) {
		return apperrors.Invalid("invalid email")
	}
	if s.prTokens == nil {
		return apperrors.Internal("password reset not configured")
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if apperrors.Is(err, apperrors.KindNotFound) {
			// Anti-enumeration: flatten the timing difference between
			// "no such user" and the happy path's hashing + insert + publish.
			time.Sleep(unknownEmailSleep)
			return nil
		}
		return err
	}

	// Invalidate any still-active tokens before issuing a new one. This
	// closes the window where an old token (e.g. one scraped from a
	// previously-leaked mailbox) stays usable after the user explicitly
	// asked for a fresh one.
	if err := s.prTokens.InvalidateAllForUser(ctx, user.ID); err != nil {
		return err
	}

	raw, hash, err := generateResetToken()
	if err != nil {
		return apperrors.Internal("generate token")
	}
	expiresAt := s.now().Add(s.prTTL).UTC()
	if _, err := s.prTokens.Create(ctx, user.ID, hash, expiresAt); err != nil {
		return err
	}

	s.publishPasswordResetRequested(ctx, user, raw, expiresAt)
	return nil
}

// ConfirmPasswordReset finalises the reset: validates the token, swaps the
// password hash, marks the token used, and bumps last_logout_at so existing
// JWTs become invalid.
func (s *Service) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return apperrors.Invalid("token is required")
	}
	if len(newPassword) < minPasswordLen {
		return apperrors.Invalid(fmt.Sprintf("password must be at least %d characters", minPasswordLen))
	}
	if s.prTokens == nil || s.prUsers == nil {
		return apperrors.Internal("password reset not configured")
	}

	hash, err := hashTokenHex(rawToken)
	if err != nil {
		// Malformed (non-hex) token — treat as unknown to avoid leaking
		// whether the format was right.
		return apperrors.Unauthorized("invalid_token")
	}

	row, err := s.prTokens.GetByHash(ctx, hash)
	if err != nil {
		if apperrors.Is(err, apperrors.KindNotFound) {
			return apperrors.Unauthorized("invalid_token")
		}
		return err
	}
	if row.UsedAt != nil {
		return apperrors.Unauthorized("invalid_token")
	}
	if !row.ExpiresAt.After(s.now()) {
		return apperrors.Unauthorized("invalid_token")
	}

	passwordHash, err := hasher.Hash(newPassword, s.cfg.Bcrypt.Cost)
	if err != nil {
		return apperrors.Internal("hash password")
	}

	// We don't wrap the two writes in a tx: the users update + token
	// invalidation are both idempotent (re-running on retry is harmless),
	// and the existing repo layer keeps DB access cleanly separated by
	// table. Mark-used last so a crash between the two leaves the token
	// re-usable but the new password already saved (the user can simply
	// log in normally).
	if err := s.prUsers.ResetPassword(ctx, row.UserID, passwordHash); err != nil {
		return err
	}
	if err := s.prTokens.MarkUsed(ctx, row.TokenID); err != nil {
		// Already-used (NotFound) should not happen in this branch (we
		// just verified used_at IS NULL above), but be tolerant.
		if !apperrors.Is(err, apperrors.KindNotFound) {
			return err
		}
	}
	return nil
}

// generateResetToken returns (raw_hex_token, sha256_hash). The raw hex token
// is what the user clicks; only its hash is persisted.
func generateResetToken() (string, []byte, error) {
	buf := make([]byte, tokenByteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw := hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

// hashTokenHex turns an incoming hex token into its sha256 hash. Returns
// an error for an empty token; non-hex strings still produce a valid hash
// (sha256 is over the raw string bytes) so they simply don't match any row.
func hashTokenHex(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty token")
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

// publishPasswordResetRequested is best-effort: if RabbitMQ is down we log
// the failure but do NOT surface an error to the HTTP caller — the user
// can request another reset, and we don't want a transient publish issue
// to look like a server error.
func (s *Service) publishPasswordResetRequested(ctx context.Context, user *models.User, rawToken string, expiresAt time.Time) {
	if s.prPub == nil {
		return
	}
	payload := events.PasswordResetRequested{
		UserID:    user.ID,
		UserEmail: user.Email,
		UserName:  strings.TrimSpace(user.FirstName + " " + user.LastName),
		ResetURL:  s.buildResetURL(rawToken),
		ExpiresAt: expiresAt,
	}
	env, err := events.NewEnvelope(events.TypePasswordResetRequested, payload)
	if err != nil {
		s.prLog.Warn("password_reset.events: build envelope", zap.Error(err))
		return
	}
	if err := s.prPub.Publish(ctx, env); err != nil {
		s.prLog.Warn("password_reset.events: publish",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
	}
}

// buildResetURL composes the SPA confirm-page link. Use URL escaping on the
// token even though it's hex (defense in depth — a future change to the
// token alphabet must not introduce a URL-injection vector).
func (s *Service) buildResetURL(rawToken string) string {
	base := s.prFrontendBaseURL
	if base == "" {
		base = "http://localhost:5173"
	}
	q := url.Values{}
	q.Set("token", rawToken)
	return base + "/reset-password?" + q.Encode()
}

// looksLikeEmail is a cheap shape check; the heavy lifting (RFC 5322) is
// out of scope. The handler layer already runs go-playground/validator with
// `email`, this is just a service-layer defense-in-depth.
func looksLikeEmail(s string) bool {
	if len(s) < 3 || len(s) > 255 {
		return false
	}
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.Contains(s, " ") {
		return false
	}
	return true
}
