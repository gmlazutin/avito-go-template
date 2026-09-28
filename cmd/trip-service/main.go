package main

import (
	"context"
	"errors"
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fatal("load configuration", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		fatal("initialize application", err)
	}
	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fatal("run application", err)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, log.Error(err))
	os.Exit(1)
}
