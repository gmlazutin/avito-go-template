package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/gmlazutin/avito-go-template/internal/app/log"
)

type Config struct {
	HTTPAddr               string        `env:"HTTP_ADDR,notEmpty"`
	HTTPReadTimeout        time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"10s"`
	HTTPReadHeaderTimeout  time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s"`
	HTTPWriteTimeout       time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s"`
	HTTPIdleTimeout        time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	LogLevel               log.Level     `env:"LOG_LEVEL,notEmpty"`
	ShutdownTimeout        time.Duration `env:"SHUTDOWN_TIMEOUT,notEmpty"`
	DatabaseURL            string        `env:"DATABASE_URL,notEmpty"`
	DatabaseMaxConns       int32         `env:"DATABASE_MAX_CONNS,notEmpty"`
	DatabaseMinConns       int32         `env:"DATABASE_MIN_CONNS,notEmpty"`
	DatabaseMaxLifetime    time.Duration `env:"DATABASE_MAX_CONN_LIFETIME,notEmpty"`
	DatabaseConnectTimeout time.Duration `env:"DATABASE_CONNECT_TIMEOUT,notEmpty"`
	DatabaseQueryTimeout   time.Duration `env:"DATABASE_QUERY_TIMEOUT,notEmpty"`
}

func Load() (*Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{
		SetDefaultsForZeroValuesOnly: true,
		UseFieldNameByDefault:        false,
	})
	if err != nil {
		return nil, fmt.Errorf("load config from environment: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func (cfg Config) validate() error {
	if cfg.DatabaseMinConns < 0 {
		return fmt.Errorf("DATABASE_MIN_CONNS must be non-negative")
	}
	if cfg.DatabaseMaxConns <= 0 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be positive")
	}
	if cfg.DatabaseMinConns > cfg.DatabaseMaxConns {
		return fmt.Errorf("DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS")
	}

	durations := map[string]time.Duration{
		"HTTP_READ_TIMEOUT":          cfg.HTTPReadTimeout,
		"HTTP_READ_HEADER_TIMEOUT":   cfg.HTTPReadHeaderTimeout,
		"HTTP_WRITE_TIMEOUT":         cfg.HTTPWriteTimeout,
		"HTTP_IDLE_TIMEOUT":          cfg.HTTPIdleTimeout,
		"SHUTDOWN_TIMEOUT":           cfg.ShutdownTimeout,
		"DATABASE_MAX_CONN_LIFETIME": cfg.DatabaseMaxLifetime,
		"DATABASE_CONNECT_TIMEOUT":   cfg.DatabaseConnectTimeout,
		"DATABASE_QUERY_TIMEOUT":     cfg.DatabaseQueryTimeout,
	}
	for name, value := range durations {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	return nil
}
