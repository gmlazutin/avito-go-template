package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gmlazutin/avito-go-template/internal/app"
	"github.com/gmlazutin/avito-go-template/internal/app/config"
	"github.com/gmlazutin/avito-go-template/internal/app/log"
)

func main() {
	slog.SetDefault(log.InitBootstrap(app.ServiceName))

	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("application failed", log.Error(err))
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

	application, err := app.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	if err := application.Run(ctx); err != nil {
		return fmt.Errorf("run application: %w", err)
	}
	return nil
}
