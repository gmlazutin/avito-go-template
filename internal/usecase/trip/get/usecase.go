package get

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
)

type UseCase struct {
	repository tripRepository
}

func New(repository tripRepository) *UseCase {
	return &UseCase{repository: repository}
}

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	value, err := uc.repository.Get(ctx, id)
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return value, nil
}
