//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/gmlazutin/avito-go-template/internal/model"
	"github.com/google/uuid"
)

type (
	tripRepository interface {
		Create(ctx context.Context, value domaintrip.Trip) (domaintrip.Trip, error)
		Get(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
		AddStatusHistory(ctx context.Context, tripID uuid.UUID, from *string, to, reason string) error
		DeleteExpiredCreateTripIdempotency(ctx context.Context, now time.Time) error
		ClaimCreateTripIdempotency(
			ctx context.Context,
			key uuid.UUID,
			requestHash []byte,
			tripID uuid.UUID,
			createdAt time.Time,
			expiresAt time.Time,
		) (bool, error)
		GetCreateTripIdempotency(ctx context.Context, key uuid.UUID) (model.CreateTripIdempotency, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	clock interface {
		Now() time.Time
	}

	idGenerator interface {
		New() uuid.UUID
	}
)
