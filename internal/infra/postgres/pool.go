package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	URL            string
	MaxConns       int32
	MinConns       int32
	MaxLifetime    time.Duration
	ConnectTimeout time.Duration
	QueryTimeout   time.Duration
	Configure      func(*pgxpool.Config)
}

func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxLifetime
	poolConfig.PingTimeout = cfg.QueryTimeout
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	poolConfig.AfterConnect = registerUTCCodec
	if cfg.Configure != nil {
		cfg.Configure(poolConfig)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	pingCtx, cancelPing := context.WithTimeout(ctx, cfg.QueryTimeout)
	defer cancelPing()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func registerUTCCodec(_ context.Context, conn *pgx.Conn) error {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})
	return nil
}
