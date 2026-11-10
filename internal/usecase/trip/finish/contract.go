//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package finish

import (
	"context"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/google/uuid"
)

type (
	tripRepository interface {
		GetForUpdate(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
		Update(ctx context.Context, value domaintrip.Trip, updatedAt time.Time) (domaintrip.Trip, error)
		AddStatusHistory(ctx context.Context, tripID uuid.UUID, from *string, to, reason string) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	clock interface {
		Now() time.Time
	}
)
