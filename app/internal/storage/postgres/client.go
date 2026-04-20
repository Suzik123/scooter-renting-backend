package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

// NewPostgres opens a pgx pool, pings it, registers a shutdown hook,
// and returns the pool plus sqlc.Queries.
func NewPostgres(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, *sqlc.Queries, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DB.PostgresDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	if cfg.DB.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(cfg.DB.MaxIdleConns)
	}
	if cfg.DB.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.DB.ConnMaxLifetime
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, sqlc.New(pool), nil
}
