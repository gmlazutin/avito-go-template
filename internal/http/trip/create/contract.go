//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
)

type useCase interface {
	Execute(ctx context.Context, input domaintrip.NewTrip, idempotencyKey *uuid.UUID) (domaintrip.Trip, bool, error)
}
