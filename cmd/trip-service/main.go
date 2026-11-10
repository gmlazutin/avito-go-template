package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gmlazutin/avito-go-template/internal/config"
	"github.com/gmlazutin/avito-go-template/internal/container"
	"github.com/gmlazutin/avito-go-template/internal/infra/telemetry"
)

func main() {
	logger := container.BootstrapLogger()
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("application failed", telemetry.Error(err))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	application, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	if err := application.Run(ctx); err != nil {
		return fmt.Errorf("run application: %w", err)
	}
	return nil
}
