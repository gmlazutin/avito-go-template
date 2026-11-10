package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type contextKey struct{}

type Manager struct {
	pool            *pgxpool.Pool
	rollbackTimeout time.Duration
}

func NewManager(pool *pgxpool.Pool, rollbackTimeout time.Duration) *Manager {
	return &Manager{pool: pool, rollbackTimeout: rollbackTimeout}
}

func (m *Manager) Do(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := transactionFromContext(ctx); ok {
		return fn(ctx)
	}
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txCtx := context.WithValue(ctx, contextKey{}, tx)
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = m.rollback(ctx, tx)
			panic(recovered)
		}
	}()
	if err := fn(txCtx); err != nil {
		if rollbackErr := m.rollback(ctx, tx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
			return fmt.Errorf("rollback transaction after %v: %w", err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func Executor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := transactionFromContext(ctx); ok {
		return tx
	}
	return pool
}

func transactionFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(contextKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

func (m *Manager) rollback(ctx context.Context, tx pgx.Tx) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.rollbackTimeout)
	defer cancel()
	return tx.Rollback(rollbackCtx)
}
