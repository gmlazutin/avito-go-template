package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gmlazutin/avito-go-template/internal/apierror"
	"github.com/gmlazutin/avito-go-template/internal/db/postgres"
	"github.com/gmlazutin/avito-go-template/internal/model"
	"github.com/google/uuid"
)

const idempotencyTTL = 24 * time.Hour

type TxManager interface {
	Do(context.Context, func(context.Context) error) error
}

type Service struct {
	repository *postgres.Repository
	txManager  TxManager
}

func New(repository *postgres.Repository, txManager TxManager) *Service {
	return &Service{
		repository: repository,
		txManager:  txManager,
	}
}

func (s *Service) Create(ctx context.Context, input model.NewTrip, idempotencyKey *uuid.UUID) (model.Trip, bool, error) {
	now := time.Now().UTC()
	trip := model.Trip{
		ID:         uuid.New(),
		UserID:     input.UserID,
		DriverID:   input.DriverID,
		StartPoint: input.StartPoint,
		EndPoint:   input.EndPoint,
		Price:      input.Price,
		Status:     "active",
		StartedAt:  now,
	}
	var created bool
	var requestHash []byte
	if idempotencyKey != nil {
		var err error
		requestHash, err = createTripHash(input)
		if err != nil {
			return model.Trip{}, false, apierror.Wrap(
				apierror.CodeInternalError,
				fmt.Errorf("hash create trip request: %w", err),
			)
		}
	}
	if err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if idempotencyKey != nil {
			if err := s.repository.DeleteExpiredCreateTripIdempotency(txCtx, now); err != nil {
				return err
			}
			claimed, err := s.repository.ClaimCreateTripIdempotency(
				txCtx,
				*idempotencyKey,
				requestHash,
				trip.ID,
				now,
				now.Add(idempotencyTTL),
			)
			if err != nil {
				return err
			}
			if !claimed {
				stored, err := s.repository.GetCreateTripIdempotency(txCtx, *idempotencyKey)
				if err != nil {
					return err
				}
				if !bytes.Equal(stored.RequestHash, requestHash) {
					return model.ErrIdempotencyConflict
				}
				existingTrip, err := s.repository.Get(txCtx, stored.TripID)
				if err != nil {
					return err
				}
				trip = existingTrip
				return nil
			}
		}
		createdTrip, err := s.repository.Create(txCtx, trip)
		if err != nil {
			return err
		}
		if err := s.repository.AddStatusHistory(txCtx, createdTrip.ID, nil, "active", "trip created"); err != nil {
			return err
		}
		trip = createdTrip
		created = true
		return nil
	}); err != nil {
		wrapped := fmt.Errorf("create trip: %w", err)
		switch {
		case errors.Is(err, model.ErrDriverBusy):
			return model.Trip{}, false, apierror.Wrap(apierror.CodeDriverBusy, wrapped).
				Msgf("Driver %s already has an active trip", input.DriverID)
		case errors.Is(err, model.ErrIdempotencyConflict):
			return model.Trip{}, false, apierror.Wrap(apierror.CodeIdempotencyConflict, wrapped)
		default:
			return model.Trip{}, false, apierror.Wrap(apierror.CodeInternalError, wrapped)
		}
	}
	return trip, created, nil
}

func createTripHash(input model.NewTrip) ([]byte, error) {
	type coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	payload := struct {
		UserID     uuid.UUID   `json:"user_id"`
		DriverID   uuid.UUID   `json:"driver_id"`
		StartPoint coordinates `json:"start_point"`
		EndPoint   coordinates `json:"end_point"`
		Price      int64       `json:"price"`
	}{
		UserID:   input.UserID,
		DriverID: input.DriverID,
		StartPoint: coordinates{
			Latitude:  input.StartPoint.Latitude,
			Longitude: input.StartPoint.Longitude,
		},
		EndPoint: coordinates{
			Latitude:  input.EndPoint.Latitude,
			Longitude: input.EndPoint.Longitude,
		},
		Price: input.Price,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	return hash[:], nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	trip, err := s.repository.Get(ctx, id)
	if err != nil {
		wrapped := fmt.Errorf("get trip: %w", err)
		if errors.Is(err, model.ErrTripNotFound) {
			return model.Trip{}, apierror.Wrap(apierror.CodeTripNotFound, wrapped).
				Msgf("Trip %s was not found", id)
		}
		return model.Trip{}, apierror.Wrap(apierror.CodeInternalError, wrapped)
	}
	return trip, nil
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (trip model.Trip, err error) {
	err = s.txManager.Do(ctx, func(txCtx context.Context) error {
		trip, err = s.repository.Finish(txCtx, id, time.Now().UTC())
		if err != nil {
			return err
		}
		from := "active"
		return s.repository.AddStatusHistory(txCtx, id, &from, "completed", "trip finished")
	})
	if err != nil {
		wrapped := fmt.Errorf("finish trip: %w", err)
		switch {
		case errors.Is(err, model.ErrTripNotFound):
			return model.Trip{}, apierror.Wrap(apierror.CodeTripNotFound, wrapped).
				Msgf("Trip %s was not found", id)
		case errors.Is(err, model.ErrTripCompleted):
			return model.Trip{}, apierror.Wrap(apierror.CodeTripCompleted, wrapped).
				Msgf("Trip %s is already completed", id)
		default:
			return model.Trip{}, apierror.Wrap(apierror.CodeInternalError, wrapped)
		}
	}
	return trip, nil
}
