package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/gmlazutin/avito-go-template/internal/infra/postgres/transaction"
)

const tripColumns = "id, user_id, driver_id, start_latitude, start_longitude, " +
	"end_latitude, end_longitude, price, status, started_at, finished_at"

func (r *Repository) Create(ctx context.Context, value domaintrip.Trip) (domaintrip.Trip, error) {
	state := value.State()
	query, args, err := sqlBuilder.Insert("trips").
		Columns(
			"id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", "status", "started_at", "finished_at",
		).
		Values(
			state.ID, state.UserID, state.DriverID, state.StartPoint.Latitude, state.StartPoint.Longitude,
			state.EndPoint.Latitude, state.EndPoint.Longitude, state.Price, state.Status, state.StartedAt, state.FinishedAt,
		).
		Suffix("RETURNING " + tripColumns).
		ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build insert trip: %w", err)
	}
	created, err := r.queryTrip(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver_idx" {
			return domaintrip.Trip{}, domaintrip.ErrDriverBusy
		}
		return domaintrip.Trip{}, fmt.Errorf("insert trip: %w", err)
	}
	return created, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	return r.get(ctx, id, false)
}

func (r *Repository) GetForUpdate(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	return r.get(ctx, id, true)
}

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	query, args, err := sqlBuilder.Select("count(*)").
		From("trips").
		Where(squirrel.Eq{"status": domaintrip.StatusActive}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count active trips: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var count int64
	if err := transaction.Executor(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}

func (r *Repository) get(ctx context.Context, id uuid.UUID, forUpdate bool) (domaintrip.Trip, error) {
	builder := sqlBuilder.Select(tripColumns).From("trips").Where(squirrel.Eq{"id": id})
	if forUpdate {
		builder = builder.Suffix("FOR UPDATE")
	}
	query, args, err := builder.ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build get trip: %w", err)
	}
	value, err := r.queryTrip(ctx, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, domaintrip.ErrTripNotFound
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return value, nil
}

func (r *Repository) Update(
	ctx context.Context,
	value domaintrip.Trip,
	updatedAt time.Time,
) (domaintrip.Trip, error) {
	state := value.State()
	query, args, err := sqlBuilder.Update("trips").
		Set("status", state.Status).
		Set("finished_at", state.FinishedAt).
		Set("updated_at", updatedAt).
		Where(squirrel.Eq{"id": state.ID}).
		Suffix("RETURNING " + tripColumns).
		ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build update trip: %w", err)
	}
	updated, err := r.queryTrip(ctx, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, domaintrip.ErrTripNotFound
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("update trip: %w", err)
	}
	return updated, nil
}

func (r *Repository) AddStatusHistory(
	ctx context.Context,
	tripID uuid.UUID,
	from *string,
	to string,
	reason string,
) error {
	query, args, err := sqlBuilder.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(tripID, from, to, reason).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if _, err := transaction.Executor(ctx, r.pool).Exec(queryCtx, query, args...); err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}

func (r *Repository) queryTrip(ctx context.Context, query string, args ...any) (domaintrip.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	return scanTrip(transaction.Executor(ctx, r.pool).QueryRow(queryCtx, query, args...))
}

func scanTrip(row pgx.Row) (domaintrip.Trip, error) {
	var state domaintrip.State
	if err := row.Scan(
		&state.ID,
		&state.UserID,
		&state.DriverID,
		&state.StartPoint.Latitude,
		&state.StartPoint.Longitude,
		&state.EndPoint.Latitude,
		&state.EndPoint.Longitude,
		&state.Price,
		&state.Status,
		&state.StartedAt,
		&state.FinishedAt,
	); err != nil {
		return domaintrip.Trip{}, err
	}
	value, err := domaintrip.Restore(state)
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("restore trip model: %w", err)
	}
	return value, nil
}
