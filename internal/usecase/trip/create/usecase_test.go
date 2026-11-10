package create_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/gmlazutin/avito-go-template/internal/model"
	"github.com/gmlazutin/avito-go-template/internal/usecase/trip/create"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestUseCaseExecuteSuccess(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	fixture.expectTransaction()
	fixture.repository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, value domaintrip.Trip) (domaintrip.Trip, error) { return value, nil },
	)
	fixture.repository.EXPECT().AddStatusHistory(
		gomock.Any(), fixture.tripID, nil, string(domaintrip.StatusActive), "trip created",
	).Return(nil)
	fixture.metrics.EXPECT().TripCreated(gomock.Any())
	fixture.logger.EXPECT().LogAttrs(gomock.Any(), slog.LevelInfo, "trip created", gomock.Any())

	result, created, err := fixture.useCase.Execute(context.Background(), fixture.input, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !created {
		t.Fatal("expected trip to be created")
	}
	if result.State().ID != fixture.tripID {
		t.Fatalf("expected trip %s, got %s", fixture.tripID, result.State().ID)
	}
}

func TestUseCaseExecuteDriverBusy(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	fixture.expectTransaction()
	fixture.repository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{}, domaintrip.ErrDriverBusy)

	_, _, err := fixture.useCase.Execute(context.Background(), fixture.input, nil)
	if !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("expected ErrDriverBusy, got %v", err)
	}
}

func TestUseCaseExecuteValidationError(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	fixture.input.UserID = uuid.Nil

	_, _, err := fixture.useCase.Execute(context.Background(), fixture.input, nil)
	var validationErr *domaintrip.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "user_id" {
		t.Fatalf("expected user_id ValidationError, got %v", err)
	}
}

func TestUseCaseExecuteIdempotentReplay(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	fixture.expectTransaction()
	key := uuid.New()
	existing := mustTrip(t, uuid.New(), fixture.input, fixture.now.Add(-time.Hour))
	var requestHash []byte

	fixture.repository.EXPECT().DeleteExpiredCreateTripIdempotency(gomock.Any(), fixture.now).Return(nil)
	fixture.repository.EXPECT().ClaimCreateTripIdempotency(
		gomock.Any(), key, gomock.Any(), fixture.tripID, fixture.now, fixture.now.Add(24*time.Hour),
	).DoAndReturn(func(_ context.Context, _ uuid.UUID, hash []byte, _ uuid.UUID, _, _ time.Time) (bool, error) {
		requestHash = append([]byte(nil), hash...)
		return false, nil
	})
	fixture.repository.EXPECT().GetCreateTripIdempotency(gomock.Any(), key).DoAndReturn(
		func(context.Context, uuid.UUID) (model.CreateTripIdempotency, error) {
			return model.CreateTripIdempotency{RequestHash: requestHash, TripID: existing.State().ID}, nil
		},
	)
	fixture.repository.EXPECT().Get(gomock.Any(), existing.State().ID).Return(existing, nil)

	result, created, err := fixture.useCase.Execute(context.Background(), fixture.input, &key)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if created {
		t.Fatal("idempotent replay must not report a new trip")
	}
	if result.State().ID != existing.State().ID {
		t.Fatalf("expected existing trip %s, got %s", existing.State().ID, result.State().ID)
	}
}

func TestUseCaseExecuteIdempotencyConflict(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	fixture.expectTransaction()
	key := uuid.New()
	fixture.repository.EXPECT().DeleteExpiredCreateTripIdempotency(gomock.Any(), fixture.now).Return(nil)
	fixture.repository.EXPECT().ClaimCreateTripIdempotency(
		gomock.Any(), key, gomock.Any(), fixture.tripID, fixture.now, fixture.now.Add(24*time.Hour),
	).Return(false, nil)
	fixture.repository.EXPECT().GetCreateTripIdempotency(gomock.Any(), key).Return(model.CreateTripIdempotency{
		RequestHash: []byte("different request"),
		TripID:      uuid.New(),
	}, nil)

	_, _, err := fixture.useCase.Execute(context.Background(), fixture.input, &key)
	if !errors.Is(err, domaintrip.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}

func TestUseCaseExecuteRollsBackOnHistoryError(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	fixture.expectTransaction()
	wantErr := errors.New("history unavailable")
	fixture.repository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, value domaintrip.Trip) (domaintrip.Trip, error) { return value, nil },
	)
	fixture.repository.EXPECT().AddStatusHistory(gomock.Any(), fixture.tripID, nil, gomock.Any(), gomock.Any()).Return(wantErr)

	_, _, err := fixture.useCase.Execute(context.Background(), fixture.input, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected history error, got %v", err)
	}
}

type fixture struct {
	repository  *MocktripRepository
	txManager   *MocktxManager
	clock       *Mockclock
	idGenerator *MockidGenerator
	logger      *Mocklogger
	metrics     *MocktripMetrics
	useCase     *create.UseCase
	now         time.Time
	tripID      uuid.UUID
	input       domaintrip.NewTrip
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	repository := NewMocktripRepository(ctrl)
	txManager := NewMocktxManager(ctrl)
	clock := NewMockclock(ctrl)
	idGenerator := NewMockidGenerator(ctrl)
	logger := NewMocklogger(ctrl)
	metrics := NewMocktripMetrics(ctrl)
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	tripID := uuid.New()
	clock.EXPECT().Now().Return(now)
	idGenerator.EXPECT().New().Return(tripID)
	return &fixture{
		repository:  repository,
		txManager:   txManager,
		clock:       clock,
		idGenerator: idGenerator,
		logger:      logger,
		metrics:     metrics,
		useCase:     create.New(repository, txManager, clock, idGenerator, logger, metrics),
		now:         now,
		tripID:      tripID,
		input: domaintrip.NewTrip{
			UserID:     uuid.New(),
			DriverID:   uuid.New(),
			StartPoint: domaintrip.Coordinates{Latitude: 59.9398, Longitude: 30.3146},
			EndPoint:   domaintrip.Coordinates{Latitude: 59.929, Longitude: 30.3626},
			Price:      1450,
		},
	}
}

func (f *fixture) expectTransaction() {
	f.txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
}

func mustTrip(t *testing.T, id uuid.UUID, input domaintrip.NewTrip, startedAt time.Time) domaintrip.Trip {
	t.Helper()
	value, err := domaintrip.New(id, input, startedAt)
	if err != nil {
		t.Fatalf("create fixture trip: %v", err)
	}
	return value
}
