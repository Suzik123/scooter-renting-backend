//go:build integration

package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/uniscoot/scooter-renting-backend/app/pkg/cache"
)

// startRedis spins up a redis:7-alpine testcontainer and returns a client.
func startRedis(t *testing.T) (*goredis.Client, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := tcredis.Run(ctx, "redis:7-alpine", testcontainers.WithEnv(nil))
	require.NoError(t, err)
	url, err := c.ConnectionString(ctx)
	require.NoError(t, err)
	opts, err := goredis.ParseURL(url)
	require.NoError(t, err)
	rdb := goredis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, rdb.Ping(pingCtx).Err())
	return rdb, func() {
		_ = rdb.Close()
		_ = c.Terminate(ctx)
	}
}

func TestJTIBlacklist_RevokeAndCheck(t *testing.T) {
	rdb, stop := startRedis(t)
	defer stop()
	bl := cache.NewJTIBlacklist(rdb)

	jti := uuid.NewString()
	ctx := context.Background()

	revoked, err := bl.IsRevoked(ctx, jti)
	require.NoError(t, err)
	require.False(t, revoked)

	require.NoError(t, bl.Revoke(ctx, jti, time.Minute))
	revoked, err = bl.IsRevoked(ctx, jti)
	require.NoError(t, err)
	require.True(t, revoked)
}

func TestJTIBlacklist_TTLExpires(t *testing.T) {
	rdb, stop := startRedis(t)
	defer stop()
	bl := cache.NewJTIBlacklist(rdb)

	jti := uuid.NewString()
	ctx := context.Background()
	require.NoError(t, bl.Revoke(ctx, jti, 1*time.Second))
	revoked, err := bl.IsRevoked(ctx, jti)
	require.NoError(t, err)
	require.True(t, revoked)

	time.Sleep(1500 * time.Millisecond)
	revoked, err = bl.IsRevoked(ctx, jti)
	require.NoError(t, err)
	require.False(t, revoked)
}
