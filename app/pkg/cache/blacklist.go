package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// jtiKeyPrefix is the Redis key prefix used by JTIBlacklist.
const jtiKeyPrefix = "jwt:revoked:"

// JTIBlacklist is a thin wrapper around go-redis that tracks revoked JWT
// identifiers. A revocation is a key in Redis whose TTL matches the residual
// lifetime of the token, so the key disappears naturally once the token
// would have expired anyway.
type JTIBlacklist struct {
	rdb *redis.Client
}

// NewJTIBlacklist constructs a JTIBlacklist over the provided client.
func NewJTIBlacklist(rdb *redis.Client) *JTIBlacklist {
	return &JTIBlacklist{rdb: rdb}
}

// Revoke marks the given jti as revoked for the given residual TTL. If ttl is
// non-positive the call is a no-op — the token is already expired.
func (b *JTIBlacklist) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if b == nil || b.rdb == nil {
		return errors.New("blacklist not configured")
	}
	if jti == "" {
		return errors.New("empty jti")
	}
	if ttl <= 0 {
		return nil
	}
	if err := b.rdb.Set(ctx, jtiKeyPrefix+jti, "1", ttl).Err(); err != nil {
		return fmt.Errorf("blacklist set: %w", err)
	}
	return nil
}

// IsRevoked reports whether the given jti is currently in the blacklist.
func (b *JTIBlacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if b == nil || b.rdb == nil {
		return false, errors.New("blacklist not configured")
	}
	if jti == "" {
		return false, nil
	}
	n, err := b.rdb.Exists(ctx, jtiKeyPrefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("blacklist exists: %w", err)
	}
	return n > 0, nil
}
