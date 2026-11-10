//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	infrapostgres "github.com/gmlazutin/avito-go-template/internal/infra/postgres"
	"github.com/gmlazutin/avito-go-template/internal/infra/postgres/transaction"
	triprepository "github.com/gmlazutin/avito-go-template/internal/repository/trip"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationTimeout = 5 * time.Second

func TestRepositoryCreateAndGet(t *testing.T) {
	t.Parallel()

	pool, repository, txManager := newRepository(t)
	value := newTrip(t, uuid.New())
	cleanupTrip(t, pool, value.State().ID)

	err := txManager.Do(context.Background(), func(ctx context.Context) error {
		created, err := repository.Create(ctx, value)
		if err != nil {
			return err
		}
		return repository.AddStatusHistory(
			ctx,
			created.State().ID,
			nil,
			string(domaintrip.StatusActive),
			"integration test",
		)
	})
	if err != nil {
		t.Fatalf("create trip transaction: %v", err)
	}

	result, err := repository.Get(context.Background(), value.State().ID)
	if err != nil {
		t.Fatalf("get trip: %v", err)
	}
	if result.State().ID != value.State().ID || result.State().Status != domaintrip.StatusActive {
		t.Fatalf("unexpected trip: %+v", result.State())
	}
}

func TestRepositoryDriverBusy(t *testing.T) {
	t.Parallel()

	pool, repository, _ := newRepository(t)
	driverID := uuid.New()
	first := newTrip(t, driverID)
	second := newTrip(t, driverID)
	cleanupTrip(t, pool, first.State().ID)
	cleanupTrip(t, pool, second.State().ID)

	if _, err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("create first trip: %v", err)
	}
	if _, err := repository.Create(context.Background(), second); !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("expected ErrDriverBusy, got %v", err)
	}
}

func TestRepositoryFinishTransaction(t *testing.T) {
	t.Parallel()

	pool, repository, txManager := newRepository(t)
	value := newTrip(t, uuid.New())
	cleanupTrip(t, pool, value.State().ID)
	if _, err := repository.Create(context.Background(), value); err != nil {
		t.Fatalf("create trip: %v", err)
	}
	finishedAt := time.Date(2026, time.November, 10, 13, 0, 0, 0, time.UTC)
	err := txManager.Do(context.Background(), func(ctx context.Context) error {
		locked, err := repository.GetForUpdate(ctx, value.State().ID)
		if err != nil {
			return err
		}
		if err := locked.Finish(finishedAt); err != nil {
			return err
		}
		if _, err := repository.Update(ctx, locked, finishedAt); err != nil {
			return err
		}
		from := string(domaintrip.StatusActive)
		return repository.AddStatusHistory(
			ctx,
			value.State().ID,
			&from,
			string(domaintrip.StatusCompleted),
			"integration test",
		)
	})
	if err != nil {
		t.Fatalf("finish transaction: %v", err)
	}

	result, err := repository.Get(context.Background(), value.State().ID)
	if err != nil {
		t.Fatalf("get finished trip: %v", err)
	}
	if result.State().Status != domaintrip.StatusCompleted || result.State().FinishedAt == nil {
		t.Fatalf("trip was not completed: %+v", result.State())
	}
}

func TestTransactionRollsBackBothTables(t *testing.T) {
	t.Parallel()

	pool, repository, txManager := newRepository(t)
	value := newTrip(t, uuid.New())
	cleanupTrip(t, pool, value.State().ID)

	err := txManager.Do(context.Background(), func(ctx context.Context) error {
		if _, err := repository.Create(ctx, value); err != nil {
			return err
		}
		return repository.AddStatusHistory(ctx, value.State().ID, nil, "invalid-status", "force rollback")
	})
	if err == nil {
		t.Fatal("expected status history constraint error")
	}
	if _, err := repository.Get(context.Background(), value.State().ID); !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("trip insert was not rolled back: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	var historyCount int
	if err := pool.QueryRow(
		ctx,
		"SELECT count(*) FROM trip_status_history WHERE trip_id = $1",
		value.State().ID,
	).Scan(&historyCount); err != nil {
		t.Fatalf("count status history: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("expected no history rows, got %d", historyCount)
	}
}

func newRepository(t *testing.T) (*pgxpool.Pool, *triprepository.Repository, *transaction.Manager) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}
	pool, err := infrapostgres.NewPool(context.Background(), infrapostgres.PoolConfig{
		URL:            databaseURL,
		MaxConns:       4,
		MinConns:       0,
		MaxLifetime:    time.Minute,
		ConnectTimeout: integrationTimeout,
		QueryTimeout:   integrationTimeout,
	})
	if err != nil {
		t.Fatalf("create integration pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, triprepository.NewRepository(pool, integrationTimeout), transaction.NewManager(pool, integrationTimeout)
}

func newTrip(t *testing.T, driverID uuid.UUID) domaintrip.Trip {
	t.Helper()
	value, err := domaintrip.New(uuid.New(), domaintrip.NewTrip{
		UserID:     uuid.New(),
		DriverID:   driverID,
		StartPoint: domaintrip.Coordinates{Latitude: 59.9398, Longitude: 30.3146},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.929, Longitude: 30.3626},
		Price:      1450,
	}, time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("create trip fixture: %v", err)
	}
	return value
}

func cleanupTrip(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if _, err := pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", id); err != nil {
			t.Errorf("clean up trip %s: %v", id, err)
		}
	})
}
