//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/google/uuid"
)

type useCase interface {
	Execute(ctx context.Context, input domaintrip.NewTrip, idempotencyKey *uuid.UUID) (domaintrip.Trip, bool, error)
}
