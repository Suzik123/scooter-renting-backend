//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	pmrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/payments"
	usersrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/users"
	"github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/sqlc"
)

// startPostgres spins up postgres:15 and applies the goose migrations from the
// project tree.
func startPostgres(t *testing.T) (*pgxpool.Pool, *sqlc.Queries, func()) {
	t.Helper()
	ctx := context.Background()

	c, err := tcpostgres.Run(ctx, "postgres:15",
		tcpostgres.WithDatabase("uniscoot"),
		tcpostgres.WithUsername("uniscoot"),
		tcpostgres.WithPassword("uniscoot"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Apply goose migrations.
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	require.NoError(t, goose.SetDialect("postgres"))

	_, file, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(file), "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		t.Fatalf("migrations dir not found: %v", err)
	}
	require.NoError(t, goose.Up(db, migrationsDir))
	_ = db.Close()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	q := sqlc.New(pool)
	return pool, q, func() {
		pool.Close()
		_ = c.Terminate(ctx)
	}
}

func TestUsersRepo_CreateAndGet(t *testing.T) {
	pool, q, stop := startPostgres(t)
	defer stop()

	repo := usersrepo.New(q, pool)
	ctx := context.Background()

	hash := "x"
	u := &models.User{
		Email:        "alice@example.com",
		FirstName:    "Alice",
		LastName:     "Doe",
		Status:       models.UserActive,
		Role:         models.RoleUser,
		PasswordHash: &hash,
	}
	require.NoError(t, repo.Create(ctx, u))
	require.NotEqual(t, uuid.Nil, u.ID)

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", got.Email)

	got2, err := repo.GetByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got2.ID)
}

func TestPaymentsRepo_CreateOfflineAndIdempotency(t *testing.T) {
	pool, q, stop := startPostgres(t)
	defer stop()

	users := usersrepo.New(q, pool)
	payments := pmrepo.New(q, pool)
	ctx := context.Background()

	// Need a user, scooter, price_model, and rental for FK validity.
	hash := "x"
	user := &models.User{
		Email:        "u@x.com",
		FirstName:    "U",
		Status:       models.UserActive,
		Role:         models.RoleUser,
		PasswordHash: &hash,
	}
	require.NoError(t, users.Create(ctx, user))

	scooter, err := q.CreateScooter(ctx, sqlc.CreateScooterParams{
		QrCode:       "QR-1",
		BatteryLevel: 100,
		Status:       models.ScooterAvailable,
		Model:        "M",
	})
	require.NoError(t, err)

	pm, err := q.CreatePriceModel(ctx, sqlc.CreatePriceModelParams{
		Name:           "default",
		UnlockFee:      decimal.NewFromInt(1),
		PricePerMinute: decimal.NewFromFloat(0.5),
		Currency:       "USD",
	})
	require.NoError(t, err)

	rental, err := q.CreateRental(ctx, sqlc.CreateRentalParams{
		UserID:       user.ID,
		ScooterID:    scooter.ScooterID,
		PriceModelID: pm.PriceModelID,
		Status:       models.RentalActive,
	})
	require.NoError(t, err)
	// End the rental so it can be paid for.
	endTime := time.Now().UTC()
	rental, err = q.EndRental(ctx, sqlc.EndRentalParams{
		EndTime:   &endTime,
		DistanceM: 0,
		TotalCost: decimal.NewFromInt(5),
		RentalID:  rental.RentalID,
	})
	require.NoError(t, err)
	require.Equal(t, models.RentalCompleted, rental.Status)

	approver := user.ID // self-approval just to satisfy FK
	in := pmrepo.CreateOfflineInput{
		UserID:         user.ID,
		RentalID:       rental.RentalID,
		Amount:         decimal.NewFromInt(5),
		Currency:       "USD",
		ApproverID:     approver,
		IdempotencyKey: "abc-123",
	}
	first, replay, err := payments.CreateOffline(ctx, in)
	require.NoError(t, err)
	require.False(t, replay)
	assert.Equal(t, models.PaymentMethodOffline, first.PaymentMethod)
	assert.Equal(t, models.PaymentSucceeded, first.Status)

	second, replay, err := payments.CreateOffline(ctx, in)
	require.NoError(t, err)
	assert.True(t, replay, "second call must be replay")
	assert.Equal(t, first.ID, second.ID)
}
