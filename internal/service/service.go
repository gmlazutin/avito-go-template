package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gmlazutin/avito-go-template/internal/apierror"
	"github.com/gmlazutin/avito-go-template/internal/db/postgres"
	"github.com/gmlazutin/avito-go-template/internal/model"
	"github.com/google/uuid"
)

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

func (s *Service) Create(ctx context.Context, input model.NewTrip) (model.Trip, error) {
	trip := model.Trip{
		ID:         uuid.New(),
		UserID:     input.UserID,
		DriverID:   input.DriverID,
		StartPoint: input.StartPoint,
		EndPoint:   input.EndPoint,
		Price:      input.Price,
		Status:     "active",
		StartedAt:  time.Now().UTC(),
	}
	if err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := s.repository.Create(txCtx, trip); err != nil {
			return err
		}
		return s.repository.AddStatusHistory(txCtx, trip.ID, nil, "active", "trip created")
	}); err != nil {
		wrapped := fmt.Errorf("create trip: %w", err)
		if errors.Is(err, model.ErrDriverBusy) {
			return model.Trip{}, apierror.Wrap(apierror.CodeDriverBusy, wrapped).
				Msgf("Driver %s already has an active trip", input.DriverID)
		}
		return model.Trip{}, apierror.Wrap(apierror.CodeInternalError, wrapped)
	}
	return trip, nil
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
