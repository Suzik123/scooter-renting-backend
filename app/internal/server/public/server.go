package public

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/middleware"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
)

// Params aggregates RegisterRoutes dependencies.
type Params struct {
	fx.In

	Server *fiber.App `name:"public-api"`

	Config     *config.Config
	Logger     *zap.Logger
	Middleware *middleware.Middleware
	Handler    *Handler
}

// Result is the fx.Out for NewServer, mirroring the jupbot pattern.
type Result struct {
	fx.Out
	Server *fiber.App `name:"public-api"`
}

// NewServer builds a fully-wired *fiber.App and registers lifecycle hooks
// for listen/shutdown.
func NewServer(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, mw *middleware.Middleware) (Result, error) {
	fiberCfg := fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: errorHandler(log),
	}

	app := fiber.New(fiberCfg)

	// Cross-cutting middlewares applied to every request.
	app.Use(mw.RequestID)
	app.Use(mw.LoggerMiddleware)
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(cors.New(corsConfig(cfg)))
	app.Use(mw.ContentTypeJSON)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				if err := app.Listen(cfg.Server.Address); err != nil {
					log.Error("http server exited", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			return app.Shutdown()
		},
	})

	return Result{Server: app}, nil
}

// RegisterRoutes wires every HTTP route onto the shared fiber.App.
func RegisterRoutes(p Params) {
	app := p.Server
	h := p.Handler
	mw := p.Middleware

	// Health / version
	app.Get("/health", h.Health)
	app.Get("/version", h.Version)

	// Stripe webhooks (no JWT, raw body read via c.Body()).
	app.Post("/webhooks/stripe", h.StripeWebhook)

	api := app.Group("/api")

	// Auth (public)
	auth := api.Group("/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/oauth/google", h.OAuthGoogle)

	// Scooters (public reads)
	scooters := api.Group("/scooters")
	scooters.Get("/", h.ListScooters)
	scooters.Get("/nearby", h.NearbyScooters)
	scooters.Get("/:id", h.GetScooter)

	// Zones + price models (public reads)
	api.Get("/zones", h.ListZones)
	api.Get("/price-models", h.ListPriceModels)

	// Authenticated routes.
	authed := api.Group("", mw.JWTAuth)
	authed.Post("/auth/logout", h.Logout)

	users := authed.Group("/users/:id")
	users.Get("/", h.GetUser)
	users.Put("/", h.UpdateUser)
	users.Delete("/", h.DeleteUser)
	users.Get("/rentals", h.ListUserRentals)
	users.Get("/payments", h.ListUserPayments)

	rentals := authed.Group("/rentals")
	rentals.Post("/", h.StartRental)
	rentals.Get("/:id", h.GetRental)
	rentals.Put("/:id/end", h.EndRental)

	payments := authed.Group("/payments")
	payments.Post("/setup-intent", h.CreateSetupIntent)
	payments.Get("/methods", h.ListPaymentMethods)
	payments.Delete("/methods/:pm_id", h.DetachPaymentMethod)

	// Admin routes.
	admin := api.Group("/admin", mw.JWTAuth, mw.AdminOnly)

	adminScooters := admin.Group("/scooters")
	adminScooters.Post("/", h.CreateScooter)
	adminScooters.Put("/:id", h.UpdateScooter)
	adminScooters.Delete("/:id", h.RetireScooter)
	adminScooters.Post("/:id/maintenance", h.OpenMaintenance)
	adminScooters.Get("/:id/maintenance", h.ListMaintenance)

	adminZones := admin.Group("/zones")
	adminZones.Post("/", h.CreateZone)
	adminZones.Put("/:id", h.UpdateZone)
	adminZones.Delete("/:id", h.DeleteZone)

	adminPM := admin.Group("/price-models")
	adminPM.Post("/", h.CreatePriceModel)
	adminPM.Put("/:id", h.UpdatePriceModel)
	adminPM.Delete("/:id", h.DeletePriceModel)

	admin.Delete("/rentals/:id", h.CancelRental)
	admin.Put("/maintenance/:id/close", h.CloseMaintenance)
	admin.Post("/payments/offline", h.ApproveOfflinePayment)
}

// corsConfig builds a cors.Config from the application config.
func corsConfig(cfg *config.Config) cors.Config {
	origins := cfg.Server.CORSOrigins
	allowOrigins := []string{"*"}
	if len(origins) > 0 {
		allowOrigins = origins
	}
	return cors.Config{
		AllowOrigins: allowOrigins,
		AllowMethods: []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete, fiber.MethodOptions},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "Stripe-Signature"},
	}
}

// errorHandler translates returned errors into the envelope format. APIError
// values keep their http code and body; fiber.Error values are passed through;
// everything else is reported as a 500 with the request id attached.
func errorHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		var apiErr *resp.APIError
		if errors.As(err, &apiErr) {
			return WriteError(c, apiErr)
		}

		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			apiErr := &resp.APIError{
				HTTPCode: fiberErr.Code,
				Code:     resp.CodeInternal,
				Message:  fiberErr.Message,
			}
			switch fiberErr.Code {
			case fiber.StatusBadRequest:
				apiErr.Code = resp.CodeBadRequest
			case fiber.StatusUnauthorized:
				apiErr.Code = resp.CodeUnauthorized
			case fiber.StatusForbidden:
				apiErr.Code = resp.CodeForbidden
			case fiber.StatusNotFound:
				apiErr.Code = resp.CodeNotFound
			case fiber.StatusConflict:
				apiErr.Code = resp.CodeConflict
			}
			return WriteError(c, apiErr)
		}

		log.Error("unhandled error",
			zap.Error(err),
			zap.String("path", c.Path()),
			zap.String("request_id", middleware.RequestIDFromCtx(c)),
		)
		return WriteError(c, resp.ErrInternal())
	}
}
