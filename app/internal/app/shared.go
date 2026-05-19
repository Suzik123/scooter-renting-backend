package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	googleclient "github.com/uniscoot/scooter-renting-backend/app/clients/google"
	stripeclient "github.com/uniscoot/scooter-renting-backend/app/clients/stripe"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/internal/notifications"
	authsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/auth"
	maintsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/maintenance"
	oauthsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/oauth"
	paymentsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/payments"
	pmsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/pricemodels"
	rentalsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/rentals"
	scootersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/scooters"
	usersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/users"
	zonesvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/zones"
	storagepg "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres"
	maintrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/maintenance"
	passwordresetsrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/passwordresets"
	paymentsrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/payments"
	pmrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/pricemodels"
	rentalsrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/rentals"
	scootersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/scooters"
	usersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/users"
	zonesrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/zones"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/cache"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/messaging"
)

// configModule loads the config and supplies the *config.Config singleton.
func configModule(version string) fx.Option {
	return fx.Provide(func() (*config.Config, error) {
		cfg, err := config.New()
		if err != nil {
			return nil, err
		}
		cfg.Version = version
		return cfg, nil
	})
}

// StorageModule wires Postgres, sqlc.Queries, and every repository the
// project owns. Shared between api and worker.
var StorageModule = fx.Options(
	fx.Provide(storagepg.NewPostgres),
	fx.Provide(
		func(q *sqlc.Queries, p *pgxpool.Pool) *usersrepo.Repository { return usersrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *zonesrepo.Repository { return zonesrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *pmrepo.Repository { return pmrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *scootersrepo.Repository { return scootersrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *rentalsrepo.Repository { return rentalsrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *maintrepo.Repository { return maintrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *paymentsrepo.Repository { return paymentsrepo.New(q, p) },
		func(q *sqlc.Queries, p *pgxpool.Pool) *passwordresetsrepo.Repository {
			return passwordresetsrepo.New(q, p)
		},
	),
)

// CacheModule wires Redis + the JTI blacklist.
var CacheModule = fx.Options(
	fx.Provide(cache.NewRedis),
	fx.Provide(cache.NewJTIBlacklist),
)

// MessagingModule wires the RabbitMQ client and the typed events.Publisher.
// OnStop closes the connection.
var MessagingModule = fx.Options(
	fx.Provide(messaging.NewClient),
	fx.Provide(messaging.NewPublisher),
	// Expose the messaging.Publisher as the events.Publisher interface so
	// services that depend on the abstraction get it injected.
	fx.Provide(func(p *messaging.Publisher) events.Publisher { return p }),
	fx.Invoke(func(lc fx.Lifecycle, c *messaging.Client) {
		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return c.Close()
			},
		})
	}),
)

// MailModule wires the SMTP sender used by the notifications worker.
var MailModule = fx.Options(
	fx.Provide(notifications.NewSender),
)

// passwordResetTokenAdapter bridges the concrete passwordresets.Repository
// to the narrow auth.PasswordResetTokenRepo interface. The auth package
// owns the row shape it consumes so it does not depend on storage internals.
type passwordResetTokenAdapter struct {
	repo *passwordresetsrepo.Repository
}

func (a *passwordResetTokenAdapter) Create(ctx context.Context, userID uuid.UUID, hash []byte, expiresAt time.Time) (*authsvc.PasswordResetTokenRow, error) {
	t, err := a.repo.Create(ctx, userID, hash, expiresAt)
	if err != nil {
		return nil, err
	}
	return toAuthRow(t), nil
}

func (a *passwordResetTokenAdapter) GetByHash(ctx context.Context, hash []byte) (*authsvc.PasswordResetTokenRow, error) {
	t, err := a.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	return toAuthRow(t), nil
}

func (a *passwordResetTokenAdapter) InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error {
	return a.repo.InvalidateAllForUser(ctx, userID)
}

func (a *passwordResetTokenAdapter) MarkUsed(ctx context.Context, tokenID uuid.UUID) error {
	return a.repo.MarkUsed(ctx, tokenID)
}

func toAuthRow(t *passwordresetsrepo.Token) *authsvc.PasswordResetTokenRow {
	if t == nil {
		return nil
	}
	return &authsvc.PasswordResetTokenRow{
		TokenID:   t.TokenID,
		UserID:    t.UserID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
		UsedAt:    t.UsedAt,
		CreatedAt: t.CreatedAt,
	}
}

// servicesModule wires every domain service. The auth service is wired with
// the blacklist after construction so its zero-dep test usage stays simple.
func servicesModule() fx.Option {
	return fx.Options(
		fx.Provide(
			func(cfg *config.Config) *stripeclient.Client {
				return stripeclient.New(cfg.Stripe.SecretKey, cfg.Stripe.WebhookSecret)
			},
			func(cfg *config.Config) (*googleclient.Client, error) {
				gc, err := googleclient.New(context.Background(), cfg.Google.ClientID)
				if err != nil {
					return nil, fmt.Errorf("init google client: %w", err)
				}
				return gc, nil
			},
		),
		fx.Provide(
			func(cfg *config.Config, ur *usersrepo.Repository, bl *cache.JTIBlacklist, prr *passwordresetsrepo.Repository, pub events.Publisher, log *zap.Logger) *authsvc.Service {
				s := authsvc.New(cfg, ur)
				s.SetBlacklist(bl)
				s.SetPasswordResetDeps(
					&passwordResetTokenAdapter{repo: prr},
					ur,
					pub,
					log,
					cfg.Auth.PasswordResetTTL,
					cfg.Frontend.BaseURL,
				)
				return s
			},
			func(ur *usersrepo.Repository) *usersvc.Service { return usersvc.New(ur) },
			func(zr *zonesrepo.Repository) *zonesvc.Service { return zonesvc.New(zr) },
			func(pr *pmrepo.Repository) *pmsvc.Service { return pmsvc.New(pr) },
			func(sr *scootersrepo.Repository) *scootersvc.Service { return scootersvc.New(sr) },
			func(mr *maintrepo.Repository) *maintsvc.Service { return maintsvc.New(mr) },
			func(sc *stripeclient.Client, pr *paymentsrepo.Repository, ur *usersrepo.Repository, rr *rentalsrepo.Repository, pool *pgxpool.Pool, cfg *config.Config, pub events.Publisher, log *zap.Logger) *paymentsvc.Service {
				s := paymentsvc.New(sc, pr, ur, pool, cfg, pub, log)
				s.SetRentalsRepo(rr)
				return s
			},
			func(pool *pgxpool.Pool, rr *rentalsrepo.Repository, sr *scootersrepo.Repository, pmR *pmrepo.Repository, ur *usersrepo.Repository, ps *paymentsvc.Service, zr *zonesrepo.Repository, pub events.Publisher, log *zap.Logger) *rentalsvc.Service {
				rs := rentalsvc.New(pool, rr, sr, pmR, ur, ps, pub, log)
				rs.SetZonesRepo(zr)
				return rs
			},
			func(ur *usersrepo.Repository, as *authsvc.Service, gc *googleclient.Client, cfg *config.Config) *oauthsvc.Service {
				return oauthsvc.New(ur, as, gc, cfg)
			},
		),
	)
}
