package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1"
	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	"github.com/gmlazutin/avito-go-template/internal/app/config"
	"github.com/gmlazutin/avito-go-template/internal/app/log"
	"github.com/gmlazutin/avito-go-template/internal/db/postgres"
	"github.com/gmlazutin/avito-go-template/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ServiceName = "trip-service"

type App struct {
	logger          *slog.Logger
	pool            *pgxpool.Pool
	server          *http.Server
	shutdownTimeout time.Duration
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	logger := log.Init(ServiceName, cfg.LogLevel)

	pool, err := newPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	repository := postgres.NewRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := postgres.NewTxManager(pool)
	tripService := service.New(repository, txManager)
	handler := httpv1.New(tripService, pool, cfg.DatabaseQueryTimeout, logger)
	router := chi.NewRouter()
	httpv1gen.HandlerWithOptions(handler, httpv1gen.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: handler.OpenAPIError,
	})

	return &App{
		logger: logger,
		pool:   pool,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadTimeout:       cfg.HTTPReadTimeout,
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.pool.Close()

	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info("HTTP server started", "address", a.server.Addr)
		serverErrors <- a.server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		a.logger.Info("shutdown started")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), a.shutdownTimeout)
	defer cancelShutdown()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		_ = a.server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	a.logger.Info("shutdown completed")
	return ctx.Err()
}

func newPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	poolConfig.MaxConns = cfg.DatabaseMaxConns
	poolConfig.MinConns = cfg.DatabaseMinConns
	poolConfig.MaxConnLifetime = cfg.DatabaseMaxLifetime

	connectCtx, cancelConnect := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout)
	defer cancelConnect()
	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
