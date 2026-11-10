package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/gmlazutin/avito-go-template/internal/config"
	"github.com/gmlazutin/avito-go-template/internal/http/router"
	tripcreatehandler "github.com/gmlazutin/avito-go-template/internal/http/trip/create"
	tripfinishhandler "github.com/gmlazutin/avito-go-template/internal/http/trip/finish"
	tripgethandler "github.com/gmlazutin/avito-go-template/internal/http/trip/get"
	infrapostgres "github.com/gmlazutin/avito-go-template/internal/infra/postgres"
	"github.com/gmlazutin/avito-go-template/internal/infra/postgres/transaction"
	"github.com/gmlazutin/avito-go-template/internal/infra/telemetry"
	triprepository "github.com/gmlazutin/avito-go-template/internal/repository/trip"
	tripcreate "github.com/gmlazutin/avito-go-template/internal/usecase/trip/create"
	tripfinish "github.com/gmlazutin/avito-go-template/internal/usecase/trip/finish"
	tripget "github.com/gmlazutin/avito-go-template/internal/usecase/trip/get"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ServiceName = "trip-service"

var ServiceVersion = "dev"

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

type uuidGenerator struct{}

func (uuidGenerator) New() uuid.UUID {
	return uuid.New()
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	logger := telemetry.NewLogger(ServiceName, cfg.LogLevel)
	providers, err := telemetry.NewProviders(ctx, telemetry.Config{
		ServiceName: cfg.OTelServiceName,
		Version:     ServiceVersion,
		Protocol:    cfg.OTelProtocol,
		Sampler:     cfg.OTelTracesSampler,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry: %w", err)
	}
	pgxTracer := otelpgx.NewTracer(
		otelpgx.WithTracerProvider(providers.TracerProvider()),
		otelpgx.WithMeterProvider(providers.MeterProvider()),
		otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
	)
	pool, err := infrapostgres.NewPool(ctx, infrapostgres.PoolConfig{
		URL:            cfg.DatabaseURL,
		MaxConns:       cfg.DatabaseMaxConns,
		MinConns:       cfg.DatabaseMinConns,
		MaxLifetime:    cfg.DatabaseMaxLifetime,
		ConnectTimeout: cfg.DatabaseConnectTimeout,
		QueryTimeout:   cfg.DatabaseQueryTimeout,
		Configure: func(poolConfig *pgxpool.Config) {
			poolConfig.ConnConfig.Tracer = pgxTracer
		},
	})
	if err != nil {
		shutdownProviders(ctx, providers, cfg.ShutdownTimeout)
		return nil, err
	}
	if err := otelpgx.RecordStats(pool, otelpgx.WithStatsMeterProvider(providers.MeterProvider())); err != nil {
		pool.Close()
		shutdownProviders(ctx, providers, cfg.ShutdownTimeout)
		return nil, fmt.Errorf("record PostgreSQL pool metrics: %w", err)
	}

	repository := triprepository.NewRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := transaction.NewManager(pool, cfg.DatabaseQueryTimeout)
	clock := systemClock{}
	createUseCase := tripcreate.New(repository, txManager, clock, uuidGenerator{})
	getUseCase := tripget.New(repository)
	finishUseCase := tripfinish.New(repository, txManager, clock)
	server := router.NewServer(
		tripcreatehandler.NewHandler(createUseCase),
		tripgethandler.NewHandler(getUseCase),
		tripfinishhandler.NewHandler(finishUseCase),
		pool,
		cfg.DatabaseQueryTimeout,
	)

	return &App{
		logger:    logger,
		pool:      pool,
		telemetry: providers,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router.New(server),
			ReadTimeout:       cfg.HTTPReadTimeout,
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func BootstrapLogger() *slog.Logger {
	return telemetry.NewLogger(ServiceName, slog.LevelInfo)
}

func shutdownProviders(ctx context.Context, providers *telemetry.Providers, timeout time.Duration) {
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	_ = providers.Shutdown(shutdownCtx)
}
