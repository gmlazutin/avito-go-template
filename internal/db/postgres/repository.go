package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/gmlazutin/avito-go-template/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var sqlBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (r *Repository) Create(ctx context.Context, trip model.Trip) (model.Trip, error) {
	query, args, err := sqlBuilder.Insert("trips").
		Columns("id", "user_id", "driver_id", "start_latitude", "start_longitude", "end_latitude", "end_longitude", "price", "status", "started_at").
		Values(trip.ID, trip.UserID, trip.DriverID, trip.StartPoint.Latitude, trip.StartPoint.Longitude, trip.EndPoint.Latitude, trip.EndPoint.Longitude, trip.Price, trip.Status, trip.StartedAt).
		Suffix("RETURNING id, user_id, driver_id, start_latitude, start_longitude, end_latitude, end_longitude, price, status, started_at, finished_at").
		ToSql()
	if err != nil {
		return model.Trip{}, fmt.Errorf("build insert trip: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	createdTrip, err := scanTrip(executor(ctx, r.pool).QueryRow(queryCtx, query, args...))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver_idx" {
			return model.Trip{}, model.ErrDriverBusy
		}
		return model.Trip{}, fmt.Errorf("insert trip: %w", err)
	}
	return createdTrip, nil
}

func (r *Repository) DeleteExpiredCreateTripIdempotency(ctx context.Context, now time.Time) error {
	query, args, err := sqlBuilder.Delete("trip_creation_idempotency").
		Where(squirrel.LtOrEq{"expires_at": now}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete expired create trip idempotency: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if _, err := executor(ctx, r.pool).Exec(queryCtx, query, args...); err != nil {
		return fmt.Errorf("delete expired create trip idempotency: %w", err)
	}
	return nil
}

func (r *Repository) ClaimCreateTripIdempotency(
	ctx context.Context,
	key uuid.UUID,
	requestHash []byte,
	tripID uuid.UUID,
	createdAt time.Time,
	expiresAt time.Time,
) (bool, error) {
	query, args, err := sqlBuilder.Insert("trip_creation_idempotency").
		Columns("key", "request_hash", "trip_id", "created_at", "expires_at").
		Values(key, requestHash, tripID, createdAt, expiresAt).
		Suffix("ON CONFLICT (key) DO NOTHING RETURNING key").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build claim create trip idempotency: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var claimedKey uuid.UUID
	err = executor(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(&claimedKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim create trip idempotency: %w", err)
	}
	return true, nil
}

func (r *Repository) GetCreateTripIdempotency(ctx context.Context, key uuid.UUID) (model.CreateTripIdempotency, error) {
	query, args, err := sqlBuilder.Select("request_hash", "trip_id").
		From("trip_creation_idempotency").
		Where(squirrel.Eq{"key": key}).
		ToSql()
	if err != nil {
		return model.CreateTripIdempotency{}, fmt.Errorf("build get create trip idempotency: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var idempotency model.CreateTripIdempotency
	if err := executor(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(
		&idempotency.RequestHash,
		&idempotency.TripID,
	); err != nil {
		return model.CreateTripIdempotency{}, fmt.Errorf("get create trip idempotency: %w", err)
	}
	return idempotency, nil
}

func (r *Repository) AddStatusHistory(ctx context.Context, tripID uuid.UUID, from *string, to, reason string) error {
	query, args, err := sqlBuilder.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").Values(tripID, from, to, reason).ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if _, err := executor(ctx, r.pool).Exec(queryCtx, query, args...); err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	query, args, err := sqlBuilder.Select("id", "user_id", "driver_id", "start_latitude", "start_longitude", "end_latitude", "end_longitude", "price", "status", "started_at", "finished_at").
		From("trips").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return model.Trip{}, fmt.Errorf("build get trip: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	trip, err := scanTrip(executor(ctx, r.pool).QueryRow(queryCtx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Trip{}, model.ErrTripNotFound
	}
	if err != nil {
		return model.Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return trip, nil
}

func (r *Repository) Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (model.Trip, error) {
	query, args, err := sqlBuilder.Update("trips").
		Set("status", "completed").Set("finished_at", finishedAt).Set("updated_at", finishedAt).
		Where(squirrel.Eq{"id": id, "status": "active"}).
		Suffix("RETURNING id, user_id, driver_id, start_latitude, start_longitude, end_latitude, end_longitude, price, status, started_at, finished_at").ToSql()
	if err != nil {
		return model.Trip{}, fmt.Errorf("build finish trip: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	trip, err := scanTrip(executor(ctx, r.pool).QueryRow(queryCtx, query, args...))
	if err == nil {
		return trip, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	existing, getErr := r.Get(ctx, id)
	if getErr != nil {
		return model.Trip{}, getErr
	}
	if existing.Status == "completed" {
		return model.Trip{}, model.ErrTripCompleted
	}
	return model.Trip{}, fmt.Errorf("finish trip in unexpected status %q", existing.Status)
}

func scanTrip(row pgx.Row) (model.Trip, error) {
	var trip model.Trip
	err := row.Scan(&trip.ID, &trip.UserID, &trip.DriverID, &trip.StartPoint.Latitude, &trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude, &trip.EndPoint.Longitude, &trip.Price, &trip.Status, &trip.StartedAt, &trip.FinishedAt)
	return trip, err
}
