package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/notifications"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/logger"
)

// RunWorker boots the queue worker process. It reuses the shared Core /
// Storage / Cache / Messaging / Mail modules and registers the notifications
// consumers but DOES NOT import the HTTP/Fiber server.
func RunWorker(version string) {
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
		MailModule,

		fx.Invoke(notifications.RegisterConsumers),
	}

	if err := fx.ValidateApp(options...); err != nil {
		l.Fatal("failed to validate worker fx app", zap.Error(err))
	}

	app := fx.New(options...)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		l.Fatal("failed to start worker", zap.Error(err))
	}

	l.Info("worker running; waiting for events")
	<-ctx.Done()

	if err := app.Stop(context.Background()); err != nil {
		l.Warn("failed to stop worker", zap.Error(err))
	}
}
