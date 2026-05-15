package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/middleware"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/public"
	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
	maintsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/maintenance"
	oauthsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/oauth"
	paymentsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/payments"
	pmsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/pricemodels"
	rentalsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/rentals"
	scootersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/scooters"
	usersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/users"
	zonesvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/zones"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/logger"
)

// authAdapter bridges the auth service to the middleware.AuthParser interface.
type authAdapter struct {
	svc *authsvc.Service
}

// Parse satisfies middleware.AuthParser. Surfaces (userID, role, jti).
func (a authAdapter) Parse(token string) (uuid.UUID, string, string, error) {
	c, err := a.svc.ParseFull(token)
	if err != nil {
		return uuid.Nil, "", "", err
	}
	return c.UserID, c.Role, c.JTI(), nil
}

// IsRevoked satisfies middleware.AuthParser.
func (a authAdapter) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return a.svc.IsRevoked(ctx, jti)
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

		configModule(version),

		StorageModule,
		CacheModule,
		MessagingModule,

		servicesModule(),

		fx.Provide(
			func(authS *authsvc.Service, cfg *config.Config, log *zap.Logger) *middleware.Middleware {
				return middleware.New(authAdapter{svc: authS}, cfg, log)
			},
			func(
				cfg *config.Config,
				log *zap.Logger,
				authS *authsvc.Service,
				oauthS *oauthsvc.Service,
				usersS *usersvc.Service,
				scootersS *scootersvc.Service,
				zonesS *zonesvc.Service,
				pmS *pmsvc.Service,
				rentalsS *rentalsvc.Service,
				maintS *maintsvc.Service,
				paymentsS *paymentsvc.Service,
			) *public.Handler {
				return public.NewHandler(public.Deps{
					Version:     cfg.Version,
					Auth:        authS,
					OAuth:       oauthS,
					Users:       usersS,
					Scooters:    scootersS,
					Zones:       zonesS,
					PriceModels: pmS,
					Rentals:     rentalsS,
					Maintenance: maintS,
					Payments:    paymentsS,
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
