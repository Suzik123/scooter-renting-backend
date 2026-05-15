// Package cache wires the project's Redis client and exposes small,
// purpose-built helpers built on top of it (e.g. JTI blacklist).
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
)

// NewRedis builds a *redis.Client from cfg.Redis, pings it, and registers
// an fx OnStop hook for clean shutdown.
func NewRedis(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*redis.Client, error) {
	if log == nil {
		log = zap.NewNop()
	}

	opts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	log.Info("redis connected", zap.String("url", cfg.Redis.URL))

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
