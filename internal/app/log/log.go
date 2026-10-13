package log

import (
	"log/slog"
	"os"
)

func InitBootstrap(service string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("service", service)
}

func Init(service string, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})).With("service", service)
}

func Error(err error) slog.Attr {
	return slog.String("error", err.Error())
}
