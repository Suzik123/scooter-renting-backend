package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/middleware"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/public"
	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
	maintsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/maintenance"
	pmsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/pricemodels"
	rentalsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/rentals"
	scootersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/scooters"
	usersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/users"
	zonesvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/zones"
	storagepg "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres"
	maintrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/maintenance"
	pmrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/pricemodels"
	rentalsrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/rentals"
	scootersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/scooters"
	usersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/users"
	zonesrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/zones"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/logger"
)

// authAdapter bridges the auth service to the middleware.AuthParser interface.
type authAdapter struct {
	svc *authsvc.Service
}

// Parse satisfies middleware.AuthParser.
func (a authAdapter) Parse(token string) (uuid.UUID, string, error) {
	c, err := a.svc.ParseJWT(token)
	if err != nil {
		return uuid.Nil, "", err
	}
	return c.UserID, c.Role, nil
}

// RunApi boots the API with Uber fx and blocks until an interrupt is received.
func RunApi(version string) {
	l := logger.New(os.Getenv("ENVIRONMENT"))
	defer func() { _ = l.Sync() }()

	options := []fx.Option{
		fx.Supply(l),

		fx.WithLogger(func() fxevent.Logger {
			return fxevent.NopLogger
		}),

		fx.Provide(func() (*config.Config, error) {
			cfg, err := config.New()
			if err != nil {
				return nil, err
			}
			cfg.Version = version
			return cfg, nil
		}),

		fx.Provide(
			storagepg.NewPostgres,
		),

		fx.Provide(
			func(q *sqlc.Queries, p *pgxpool.Pool) *usersrepo.Repository { return usersrepo.New(q, p) },
			func(q *sqlc.Queries, p *pgxpool.Pool) *zonesrepo.Repository { return zonesrepo.New(q, p) },
			func(q *sqlc.Queries, p *pgxpool.Pool) *pmrepo.Repository { return pmrepo.New(q, p) },
			func(q *sqlc.Queries, p *pgxpool.Pool) *scootersrepo.Repository { return scootersrepo.New(q, p) },
			func(q *sqlc.Queries, p *pgxpool.Pool) *rentalsrepo.Repository { return rentalsrepo.New(q, p) },
			func(q *sqlc.Queries, p *pgxpool.Pool) *maintrepo.Repository { return maintrepo.New(q, p) },
		),

		fx.Provide(
			func(cfg *config.Config, ur *usersrepo.Repository) *authsvc.Service { return authsvc.New(cfg, ur) },
			func(ur *usersrepo.Repository) *usersvc.Service { return usersvc.New(ur) },
			func(zr *zonesrepo.Repository) *zonesvc.Service { return zonesvc.New(zr) },
			func(pr *pmrepo.Repository) *pmsvc.Service { return pmsvc.New(pr) },
			func(sr *scootersrepo.Repository) *scootersvc.Service { return scootersvc.New(sr) },
			func(pool *pgxpool.Pool, rr *rentalsrepo.Repository, sr *scootersrepo.Repository, pmR *pmrepo.Repository, log *zap.Logger) *rentalsvc.Service {
				return rentalsvc.New(pool, rr, sr, pmR, log)
			},
			func(mr *maintrepo.Repository) *maintsvc.Service { return maintsvc.New(mr) },
		),

		fx.Provide(
			func(authS *authsvc.Service, cfg *config.Config, log *zap.Logger) *middleware.Middleware {
				return middleware.New(authAdapter{svc: authS}, cfg, log)
			},
			func(cfg *config.Config, log *zap.Logger, authS *authsvc.Service, usersS *usersvc.Service, scootersS *scootersvc.Service, zonesS *zonesvc.Service, pmS *pmsvc.Service, rentalsS *rentalsvc.Service, maintS *maintsvc.Service) *public.Handler {
				return public.NewHandler(public.Deps{
					Version:     cfg.Version,
					Auth:        authS,
					Users:       usersS,
					Scooters:    scootersS,
					Zones:       zonesS,
					PriceModels: pmS,
					Rentals:     rentalsS,
					Maintenance: maintS,
					Log:         log,
				})
			},
			public.NewServer,
		),

		fx.Invoke(public.RegisterRoutes),
	}

	if err := fx.ValidateApp(options...); err != nil {
		l.Fatal("failed to validate fx app", zap.Error(err))
	}

	app := fx.New(options...)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		l.Fatal("failed to start app", zap.Error(err))
	}

	<-ctx.Done()

	if err := app.Stop(context.Background()); err != nil {
		l.Warn("failed to stop app", zap.Error(err))
	}
}
