package create

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/google/uuid"
)

const idempotencyTTL = 24 * time.Hour

type UseCase struct {
	repository  tripRepository
	txManager   txManager
	clock       clock
	idGenerator idGenerator
}

func New(repository tripRepository, txManager txManager, clock clock, idGenerator idGenerator) *UseCase {
	return &UseCase{
		repository:  repository,
		txManager:   txManager,
		clock:       clock,
		idGenerator: idGenerator,
	}
}

func (uc *UseCase) Execute(
	ctx context.Context,
	input domaintrip.NewTrip,
	idempotencyKey *uuid.UUID,
) (domaintrip.Trip, bool, error) {
	now := uc.clock.Now().UTC()
	value, err := domaintrip.New(uc.idGenerator.New(), input, now)
	if err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("create trip model: %w", err)
	}

	requestHash, err := hashInput(input, idempotencyKey != nil)
	if err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("hash create trip request: %w", err)
	}

	created := false
	err = uc.txManager.Do(ctx, func(txCtx context.Context) error {
		if idempotencyKey != nil {
			existing, found, err := uc.resolveIdempotency(txCtx, *idempotencyKey, requestHash, value, now)
			if err != nil {
				return err
			}
			if found {
				value = existing
				return nil
			}
		}

		value, err = uc.repository.Create(txCtx, value)
		if err != nil {
			return fmt.Errorf("save trip: %w", err)
		}
		if err := uc.repository.AddStatusHistory(txCtx, value.State().ID, nil, string(domaintrip.StatusActive), "trip created"); err != nil {
			return fmt.Errorf("save initial trip status: %w", err)
		}
		created = true
		return nil
	})
	if err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("create trip: %w", err)
	}
	return value, created, nil
}

func (uc *UseCase) resolveIdempotency(
	ctx context.Context,
	key uuid.UUID,
	requestHash []byte,
	value domaintrip.Trip,
	now time.Time,
) (domaintrip.Trip, bool, error) {
	if err := uc.repository.DeleteExpiredCreateTripIdempotency(ctx, now); err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("delete expired idempotency keys: %w", err)
	}
	claimed, err := uc.repository.ClaimCreateTripIdempotency(
		ctx,
		key,
		requestHash,
		value.State().ID,
		now,
		now.Add(idempotencyTTL),
	)
	if err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("claim idempotency key: %w", err)
	}
	if claimed {
		return domaintrip.Trip{}, false, nil
	}

	stored, err := uc.repository.GetCreateTripIdempotency(ctx, key)
	if err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("get idempotency key: %w", err)
	}
	if !bytes.Equal(stored.RequestHash, requestHash) {
		return domaintrip.Trip{}, false, domaintrip.ErrIdempotencyConflict
	}
	existing, err := uc.repository.Get(ctx, stored.TripID)
	if err != nil {
		return domaintrip.Trip{}, false, fmt.Errorf("get idempotent trip: %w", err)
	}
	return existing, true, nil
}

func hashInput(input domaintrip.NewTrip, enabled bool) ([]byte, error) {
	if !enabled {
		return nil, nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	hash := sha256.Sum256(payload)
	return hash[:], nil
}
