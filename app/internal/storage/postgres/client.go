package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

// NewPostgres opens a pgx pool, pings it, applies pending goose migrations
// from the embedded FS, registers a shutdown hook, and returns the pool plus
// sqlc.Queries. Applying migrations at startup lets a fresh `docker compose
// up` produce a usable schema with no manual goose step.
func NewPostgres(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*pgxpool.Pool, *sqlc.Queries, error) {
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

	migCtx, migCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer migCancel()
	if err := RunMigrations(migCtx, pool, log); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, sqlc.New(pool), nil
}
