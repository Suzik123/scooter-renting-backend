package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies every pending goose migration embedded in the binary.
// It runs once at api startup so a fresh `docker compose up` produces a usable
// schema without a separate `goose up` step.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) error {
	if log == nil {
		log = zap.NewNop()
	}
	cfg := pool.Config().ConnConfig
	db := stdlib.OpenDB(*cfg)
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping for migrations: %w", err)
	}

	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(gooseZapLogger{log: log})
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	version, err := currentVersion(ctx, db)
	if err != nil {
		log.Warn("goose: could not read current version", zap.Error(err))
	} else {
		log.Info("migrations applied", zap.Int64("version", version))
	}
	return nil
}

func currentVersion(ctx context.Context, db *sql.DB) (int64, error) {
	return goose.GetDBVersionContext(ctx, db)
}

// gooseZapLogger adapts a zap.Logger to the goose.Logger interface so
// migration progress shows up in the structured api/worker logs.
type gooseZapLogger struct{ log *zap.Logger }

func (g gooseZapLogger) Fatalf(format string, v ...any) {
	g.log.Sugar().Fatalf("goose: "+format, v...)
}
func (g gooseZapLogger) Printf(format string, v ...any) {
	g.log.Sugar().Infof("goose: "+format, v...)
}
