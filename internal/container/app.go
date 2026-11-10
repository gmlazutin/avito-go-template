package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	logger          *slog.Logger
	pool            *pgxpool.Pool
	server          *http.Server
	shutdownTimeout time.Duration
}

func (a *App) Run(ctx context.Context) error {
	defer a.pool.Close()
	serverErrors := make(chan error, 1)
	go func() {
		a.logger.InfoContext(ctx, "HTTP server started", "address", a.server.Addr)
		serverErrors <- a.server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		a.logger.InfoContext(context.WithoutCancel(ctx), "shutdown started")
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

	a.logger.InfoContext(context.WithoutCancel(ctx), "shutdown completed")
	return ctx.Err()
}
