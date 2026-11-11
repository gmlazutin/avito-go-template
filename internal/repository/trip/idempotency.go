package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gmlazutin/avito-go-template/internal/infra/postgres/transaction"
	"github.com/gmlazutin/avito-go-template/internal/model"
)

func (r *Repository) DeleteExpiredCreateTripIdempotency(ctx context.Context, now time.Time) error {
	query, args, err := sqlBuilder.Delete("trip_creation_idempotency").
		Where(squirrel.LtOrEq{"expires_at": now}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete expired create trip idempotency: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if _, err := transaction.Executor(ctx, r.pool).Exec(queryCtx, query, args...); err != nil {
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
	err = transaction.Executor(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(&claimedKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim create trip idempotency: %w", err)
	}
	return true, nil
}

func (r *Repository) GetCreateTripIdempotency(
	ctx context.Context,
	key uuid.UUID,
) (model.CreateTripIdempotency, error) {
	query, args, err := sqlBuilder.Select("request_hash", "trip_id").
		From("trip_creation_idempotency").
		Where(squirrel.Eq{"key": key}).
		ToSql()
	if err != nil {
		return model.CreateTripIdempotency{}, fmt.Errorf("build get create trip idempotency: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var result model.CreateTripIdempotency
	if err := transaction.Executor(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(
		&result.RequestHash,
		&result.TripID,
	); err != nil {
		return model.CreateTripIdempotency{}, fmt.Errorf("get create trip idempotency: %w", err)
	}
	return result, nil
}
