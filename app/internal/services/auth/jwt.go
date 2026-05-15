package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

// Claims is the JWT payload carried between the auth service and the HTTP middleware.
type Claims struct {
	UserID uuid.UUID `json:"uid"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// JTI returns the jti (token id) embedded in the claims.
func (c *Claims) JTI() string {
	if c == nil {
		return ""
	}
	return c.ID
}

// Expiry returns the absolute expiry time embedded in the claims, or zero.
func (c *Claims) Expiry() time.Time {
	if c == nil || c.ExpiresAt == nil {
		return time.Time{}
	}
	return c.ExpiresAt.Time
}

// IssueJWT signs an HS256 token for the given user using the configured secret, issuer and TTL.
func (s *Service) IssueJWT(user *models.User) (string, error) {
	now := s.now()
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    s.cfg.JWT.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWT.TTL)),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return "", apperrors.Internal("sign jwt")
	}
	return signed, nil
}

// ParseJWT validates and decodes an HS256 JWT produced by IssueJWT.
func (s *Service) ParseJWT(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, apperrors.Unauthorized("invalid token")
	}

	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.JWT.Secret), nil
	}, jwt.WithIssuer(s.cfg.JWT.Issuer), jwt.WithExpirationRequired(), jwt.WithLeeway(5*time.Second))

	if err != nil || !parsed.Valid {
		return nil, apperrors.Unauthorized("invalid token")
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.UserID == uuid.Nil {
		return nil, apperrors.Unauthorized("invalid token")
	}
	return claims, nil
}
