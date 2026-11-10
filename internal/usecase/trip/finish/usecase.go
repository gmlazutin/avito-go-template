package finish

import (
	"context"
	"fmt"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/google/uuid"
)

type UseCase struct {
	repository tripRepository
	txManager  txManager
	clock      clock
}

func New(repository tripRepository, txManager txManager, clock clock) *UseCase {
	return &UseCase{repository: repository, txManager: txManager, clock: clock}
}

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (result domaintrip.Trip, err error) {
	finishedAt := uc.clock.Now().UTC()
	err = uc.txManager.Do(ctx, func(txCtx context.Context) error {
		value, err := uc.repository.GetForUpdate(txCtx, id)
		if err != nil {
			return fmt.Errorf("get trip for update: %w", err)
		}
		if err := value.Finish(finishedAt); err != nil {
			return fmt.Errorf("finish trip model: %w", err)
		}
		result, err = uc.repository.Update(txCtx, value, finishedAt)
		if err != nil {
			return fmt.Errorf("save finished trip: %w", err)
		}
		from := string(domaintrip.StatusActive)
		if err := uc.repository.AddStatusHistory(
			txCtx,
			id,
			&from,
			string(domaintrip.StatusCompleted),
			"trip finished",
		); err != nil {
			return fmt.Errorf("save completed trip status: %w", err)
		}
		return nil
	})
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	return result, nil
}
