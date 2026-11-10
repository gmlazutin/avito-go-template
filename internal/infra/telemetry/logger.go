package telemetry

import (
	"log/slog"
	"os"
)

func NewLogger(service string, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})).With("service", service)
}

func Error(err error) slog.Attr {
	return slog.String("error", err.Error())
}
