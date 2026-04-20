package rentals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
)

type RentalsRepo interface {
	CreateTx(ctx context.Context, tx pgx.Tx, rental *models.Rental) error
	Get(ctx context.Context, id uuid.UUID) (*models.Rental, error)
	GetForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Rental, error)
	EndTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, endedAt time.Time, distanceM int, totalCost decimal.Decimal) (*models.Rental, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Rental, int, error)
}

type ScootersRepo interface {
	SetStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, fromStatus, toStatus string) error
}

type PriceModelRepo interface {
	Get(ctx context.Context, id uuid.UUID) (*models.PriceModel, error)
}

type Service struct {
	pool        *pgxpool.Pool
	rentals     RentalsRepo
	scooters    ScootersRepo
	pricemodels PriceModelRepo
	log         *zap.Logger
}

func New(pool *pgxpool.Pool, rentals RentalsRepo, scooters ScootersRepo, pricemodels PriceModelRepo, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{
		pool:        pool,
		rentals:     rentals,
		scooters:    scooters,
		pricemodels: pricemodels,
		log:         log,
	}
}

// Start opens a rental: locks the scooter from available->rented and inserts a new active rental.
func (s *Service) Start(ctx context.Context, userID, scooterID, priceModelID uuid.UUID) (*models.Rental, error) {
	if _, err := s.pricemodels.Get(ctx, priceModelID); err != nil {
		return nil, err
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
		StartedAt:    time.Now().UTC(),
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
	return rental, nil
}

// End finalizes a rental: computes the total cost and releases the scooter.
func (s *Service) End(ctx context.Context, rentalID, actorUserID uuid.UUID) (*models.Rental, error) {
	existing, err := s.rentals.Get(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	pm, err := s.pricemodels.Get(ctx, existing.PriceModelID)
	if err != nil {
		return nil, err
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

	locked, err := s.rentals.GetForUpdateTx(ctx, tx, rentalID)
	if err != nil {
		return nil, err
	}
	if locked.UserID != actorUserID {
		return nil, apperrors.Forbidden("not rental owner")
	}
	if locked.Status != models.RentalActive {
		return nil, apperrors.RentalAlreadyEnded("")
	}

	now := time.Now().UTC()
	duration := now.Sub(locked.StartedAt)
	cost := CalculateCost(pm, duration)

	updated, err := s.rentals.EndTx(ctx, tx, rentalID, now, 0, cost)
	if err != nil {
		return nil, err
	}

	if err := s.scooters.SetStatusTx(ctx, tx, locked.ScooterID, models.ScooterRented, models.ScooterAvailable); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	committed = true
	return updated, nil
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
