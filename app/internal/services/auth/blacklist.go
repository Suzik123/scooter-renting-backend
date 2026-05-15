package auth

import (
	"context"
	"time"
)

// Blacklist is the narrow interface the auth service uses to revoke JTIs.
// Concrete impl lives in app/pkg/cache.JTIBlacklist.
type Blacklist interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// nopBlacklist is the default when the auth service is constructed without
// a Redis-backed blacklist (e.g. unit tests). All operations are no-ops and
// IsRevoked always returns false.
type nopBlacklist struct{}

func (nopBlacklist) Revoke(_ context.Context, _ string, _ time.Duration) error { return nil }
func (nopBlacklist) IsRevoked(_ context.Context, _ string) (bool, error)        { return false, nil }
