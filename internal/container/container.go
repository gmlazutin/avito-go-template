package container

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
)

const ServiceName = "trip-service"

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
	pool, err := infrapostgres.NewPool(ctx, infrapostgres.PoolConfig{
		URL:            cfg.DatabaseURL,
		MaxConns:       cfg.DatabaseMaxConns,
		MinConns:       cfg.DatabaseMinConns,
		MaxLifetime:    cfg.DatabaseMaxLifetime,
		ConnectTimeout: cfg.DatabaseConnectTimeout,
		QueryTimeout:   cfg.DatabaseQueryTimeout,
	})
	if err != nil {
		return nil, err
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
		logger: logger,
		pool:   pool,
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
