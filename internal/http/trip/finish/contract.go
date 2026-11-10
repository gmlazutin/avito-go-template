//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package finish

import (
	"context"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/google/uuid"
)

type useCase interface {
	Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
}
