package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/gmlazutin/avito-go-template/internal/app/log"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	HTTPAddr               string        `env:"HTTP_ADDR" envDefault:":8080" validate:"hostname_port"`
	HTTPReadTimeout        time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"10s" validate:"gt=0"`
	HTTPReadHeaderTimeout  time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s" validate:"gt=0"`
	HTTPWriteTimeout       time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s" validate:"gt=0"`
	HTTPIdleTimeout        time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s" validate:"gt=0"`
	LogLevel               log.Level     `env:"LOG_LEVEL" envDefault:"info"`
	ShutdownTimeout        time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s" validate:"gt=0"`
	DatabaseURL            string        `env:"DATABASE_URL" validate:"required,url"`
	DatabaseMaxConns       int32         `env:"DATABASE_MAX_CONNS" envDefault:"10" validate:"gt=0"`
	DatabaseMinConns       int32         `env:"DATABASE_MIN_CONNS" envDefault:"2" validate:"gte=0"`
	DatabaseMaxLifetime    time.Duration `env:"DATABASE_MAX_CONN_LIFETIME" envDefault:"30m" validate:"gt=0"`
	DatabaseConnectTimeout time.Duration `env:"DATABASE_CONNECT_TIMEOUT" envDefault:"5s" validate:"gt=0"`
	DatabaseQueryTimeout   time.Duration `env:"DATABASE_QUERY_TIMEOUT" envDefault:"3s" validate:"gt=0"`
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
	if err := validator.New().Struct(cfg); err != nil {
		return fmt.Errorf("validate config fields: %w", err)
	}
	if cfg.DatabaseMinConns > cfg.DatabaseMaxConns {
		return fmt.Errorf("DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS")
	}
	return nil
}
