package rentals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/pkg/geo"
)

// PaymentResult is returned by End to surface the post-rental charge outcome.
type PaymentResult struct {
	ID            uuid.UUID
	Status        string
	ClientSecret  *string
	FailureReason *string
}

type RentalsRepo interface {
	CreateTx(ctx context.Context, tx pgx.Tx, rental *models.Rental) error
	Get(ctx context.Context, id uuid.UUID) (*models.Rental, error)
	GetForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Rental, error)
	EndTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, endTime time.Time, endLat, endLon *decimal.Decimal, distanceM int, totalCost decimal.Decimal) (*models.Rental, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Rental, int, error)
}

type ScootersRepo interface {
	SetStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, fromStatus, toStatus string) error
}

type PriceModelRepo interface {
	Get(ctx context.Context, id uuid.UUID) (*models.PriceModel, error)
}

// UsersRepo loads users for payment-related preconditions/charging.
type UsersRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// PaymentsService captures the rentals' dependency on the payments domain.
type PaymentsService interface {
	UserHasPaymentMethod(ctx context.Context, userID uuid.UUID) (bool, error)
	UserHasUnpaidRentals(ctx context.Context, userID uuid.UUID) (bool, error)
	ChargeRental(ctx context.Context, rental *models.Rental, user *models.User) (*PaymentResult, error)
}

// ZonesRepo provides read-only access to the zones table for geo-fenced
// enforcement (currently only no_park on End).
type ZonesRepo interface {
	List(ctx context.Context, page models.Page) ([]models.Zone, int, error)
}

type Service struct {
	pool        *pgxpool.Pool
	rentals     RentalsRepo
	scooters    ScootersRepo
	pricemodels PriceModelRepo
	users       UsersRepo
	payments    PaymentsService
	zones       ZonesRepo
	pub         events.Publisher
	log         *zap.Logger
}

func New(pool *pgxpool.Pool, rentals RentalsRepo, scooters ScootersRepo, pricemodels PriceModelRepo, users UsersRepo, payments PaymentsService, pub events.Publisher, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	if pub == nil {
		pub = events.NopPublisher{}
	}
	return &Service{
		pool:        pool,
		rentals:     rentals,
		scooters:    scooters,
		pricemodels: pricemodels,
		users:       users,
		payments:    payments,
		pub:         pub,
		log:         log,
	}
}

// SetZonesRepo wires the zones reader after construction so tests and the
// existing fx graph keep their current arities. nil disables geo enforcement.
func (s *Service) SetZonesRepo(z ZonesRepo) {
	s.zones = z
}

// Start opens a rental: enforces payment preconditions, then locks the scooter
// from available -> rented and inserts a new active rental.
func (s *Service) Start(ctx context.Context, userID, scooterID, priceModelID uuid.UUID, startLat, startLon *decimal.Decimal) (*models.Rental, error) {
	if _, err := s.pricemodels.Get(ctx, priceModelID); err != nil {
		return nil, err
	}

	hasPM, err := s.payments.UserHasPaymentMethod(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !hasPM {
		return nil, apperrors.Conflict("add_card_required")
	}

	hasUnpaid, err := s.payments.UserHasUnpaidRentals(ctx, userID)
	if err != nil {
		return nil, err
	}
	if hasUnpaid {
		return nil, apperrors.Conflict("outstanding_balance")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := s.scooters.SetStatusTx(ctx, tx, scooterID, models.ScooterAvailable, models.ScooterRented); err != nil {
		return nil, err
	}

	rental := &models.Rental{
		UserID:       userID,
		ScooterID:    scooterID,
		PriceModelID: priceModelID,
		StartTime:    time.Now().UTC(),
		StartLat:     startLat,
		StartLon:     startLon,
		Status:       models.RentalActive,
		DistanceM:    0,
		TotalCost:    decimal.Zero,
	}
	if err := s.rentals.CreateTx(ctx, tx, rental); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	committed = true

	s.emitRentalStarted(ctx, rental)
	return rental, nil
}

// End finalizes a rental: closes the rental in a tx, releases the scooter,
// then attempts to charge the user via the payments service.
func (s *Service) End(ctx context.Context, rentalID, actorUserID uuid.UUID, endLat, endLon *decimal.Decimal) (*models.Rental, *PaymentResult, error) {
	existing, err := s.rentals.Get(ctx, rentalID)
	if err != nil {
		return nil, nil, err
	}
	pm, err := s.pricemodels.Get(ctx, existing.PriceModelID)
	if err != nil {
		return nil, nil, err
	}

	// Geo enforcement: refuse to end a ride inside a no_park zone. We can
	// only enforce what we can measure — if the client did not send
	// coordinates (permission denied / browser without geolocation), we
	// fall through and let the ride end.
	if err := s.enforceNoParkZone(ctx, endLat, endLon); err != nil {
		return nil, nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	locked, err := s.rentals.GetForUpdateTx(ctx, tx, rentalID)
	if err != nil {
		return nil, nil, err
	}
	if locked.UserID != actorUserID {
		return nil, nil, apperrors.Forbidden("not rental owner")
	}
	if locked.Status != models.RentalActive {
		return nil, nil, apperrors.RentalAlreadyEnded("")
	}

	now := time.Now().UTC()
	duration := now.Sub(locked.StartTime)
	cost := CalculateCost(pm, duration)

	updated, err := s.rentals.EndTx(ctx, tx, rentalID, now, endLat, endLon, 0, cost)
	if err != nil {
		return nil, nil, err
	}

	if err := s.scooters.SetStatusTx(ctx, tx, locked.ScooterID, models.ScooterRented, models.ScooterAvailable); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	committed = true

	user, err := s.users.GetByID(ctx, actorUserID)
	// Publish RentalCompleted regardless of whether the user lookup
	// succeeded — the event is best-effort but the rental is the
	// source of truth.
	s.emitRentalCompleted(ctx, updated, user)
	if err != nil {
		s.log.Error("rentals.End: load user for charge", zap.String("user_id", actorUserID.String()), zap.Error(err))
		reason := "user_lookup_failed"
		return updated, &PaymentResult{
			Status:        models.PaymentFailed,
			FailureReason: &reason,
		}, nil
	}

	pay, err := s.payments.ChargeRental(ctx, updated, user)
	if err != nil {
		// ChargeRental only returns err for system/programmer faults —
		// card declines and missing payment methods are reported via
		// pay.Status == 'failed'. Either way the rental stays closed,
		// and the user is reblocked on next ride via the
		// outstanding-balance precondition.
		s.log.Error("rentals.End: charge rental system error",
			zap.String("rental_id", updated.ID.String()),
			zap.String("user_id", actorUserID.String()),
			zap.Error(err),
		)
		reason := "payment_processing_error"
		return updated, &PaymentResult{
			Status:        models.PaymentFailed,
			FailureReason: &reason,
		}, nil
	}
	return updated, pay, nil
}

// AdminCancel forces an active rental to cancelled and releases the scooter.
func (s *Service) AdminCancel(ctx context.Context, rentalID uuid.UUID) error {
	rental, err := s.rentals.Get(ctx, rentalID)
	if err != nil {
		return err
	}
	if rental.Status != models.RentalActive {
		return apperrors.Conflict("rental not active")
	}
	if err := s.rentals.Cancel(ctx, rentalID); err != nil {
		return err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := s.scooters.SetStatusTx(ctx, tx, rental.ScooterID, models.ScooterRented, models.ScooterAvailable); err != nil {
		if errors.Is(err, apperrors.ErrScooterUnavailable) || apperrors.Is(err, apperrors.KindScooterUnavailable) {
			// scooter already released by a concurrent End — treat as success.
		} else {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Rental, int, error) {
	return s.rentals.ListByUser(ctx, userID, page)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.Rental, error) {
	return s.rentals.Get(ctx, id)
}

// enforceNoParkZone returns a ZoneViolation error when (lat,lon) sits inside
// any zone with type=no_park. nil coords or an unwired zones repo short-circuit
// to nil (allow). The list-and-filter approach is cheap at MVP scale
// (~tens of zones); switch to PostGIS or a sqlc spatial query if the table
// grows past a few hundred rows.
func (s *Service) enforceNoParkZone(ctx context.Context, lat, lon *decimal.Decimal) error {
	if lat == nil || lon == nil {
		s.log.Warn("rentals.End: ending without coordinates; skipping geo enforcement")
		return nil
	}
	if s.zones == nil {
		return nil
	}
	zones, _, err := s.zones.List(ctx, models.Page{Limit: 1000, Offset: 0})
	if err != nil {
		// Don't block ride end on a transient zones-table error: log and let
		// the rental close. The alternative leaves the scooter rented and
		// the user stuck.
		s.log.Error("rentals.End: list zones failed; skipping geo enforcement", zap.Error(err))
		return nil
	}
	latF, _ := lat.Float64()
	lonF, _ := lon.Float64()
	for _, z := range zones {
		if z.ZoneType != models.ZoneTypeNoPark {
			continue
		}
		if geo.IsInsideZone(latF, lonF, z) {
			return apperrors.ZoneViolation("cannot_end_in_no_park_zone")
		}
	}
	return nil
}

// emitRentalStarted publishes a RentalStarted event. Errors are logged but
// never propagated.
func (s *Service) emitRentalStarted(ctx context.Context, r *models.Rental) {
	if r == nil {
		return
	}
	payload := events.RentalStarted{
		RentalID:  r.ID,
		UserID:    r.UserID,
		ScooterID: r.ScooterID,
		StartedAt: r.StartTime,
	}
	env, err := events.NewEnvelope(events.TypeRentalStarted, payload)
	if err != nil {
		s.log.Warn("rentals.events: build envelope", zap.Error(err))
		return
	}
	if err := s.pub.Publish(ctx, env); err != nil {
		s.log.Warn("rentals.events: publish RentalStarted", zap.Error(err))
	}
}

// emitRentalCompleted publishes a RentalCompleted event. user may be nil if
// the lookup failed; the email/name fields fall back to empty strings.
func (s *Service) emitRentalCompleted(ctx context.Context, r *models.Rental, user *models.User) {
	if r == nil || r.EndTime == nil {
		return
	}
	email, name := "", ""
	if user != nil {
		email = user.Email
		name = strings.TrimSpace(user.FirstName + " " + user.LastName)
	}
	payload := events.RentalCompleted{
		RentalID:  r.ID,
		UserID:    r.UserID,
		ScooterID: r.ScooterID,
		StartedAt: r.StartTime,
		EndedAt:   *r.EndTime,
		DurationS: int64(r.EndTime.Sub(r.StartTime).Seconds()),
		DistanceM: r.DistanceM,
		TotalCost: r.TotalCost,
		Currency:  "",
		UserEmail: email,
		UserName:  name,
	}
	env, err := events.NewEnvelope(events.TypeRentalCompleted, payload)
	if err != nil {
		s.log.Warn("rentals.events: build envelope", zap.Error(err))
		return
	}
	if err := s.pub.Publish(ctx, env); err != nil {
		s.log.Warn("rentals.events: publish RentalCompleted", zap.Error(err))
	}
}
