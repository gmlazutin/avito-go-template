package finish_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	tripfinish "github.com/gmlazutin/avito-go-template/internal/usecase/trip/finish"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestUseCaseExecuteSuccess(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, activeTrip(t))
	fixture.expectTransaction()
	fixture.repository.EXPECT().GetForUpdate(gomock.Any(), fixture.value.State().ID).Return(fixture.value, nil)
	fixture.repository.EXPECT().Update(gomock.Any(), gomock.Any(), fixture.now).DoAndReturn(
		func(_ context.Context, value domaintrip.Trip, _ time.Time) (domaintrip.Trip, error) {
			return value, nil
		},
	)
	from := string(domaintrip.StatusActive)
	fixture.repository.EXPECT().AddStatusHistory(
		gomock.Any(), fixture.value.State().ID, &from, string(domaintrip.StatusCompleted), "trip finished",
	).Return(nil)
	fixture.metrics.EXPECT().TripCompleted(gomock.Any())
	fixture.logger.EXPECT().LogAttrs(gomock.Any(), slog.LevelInfo, "trip completed", gomock.Any())

	result, err := fixture.useCase.Execute(context.Background(), fixture.value.State().ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.State().Status != domaintrip.StatusCompleted {
		t.Fatalf("expected completed trip, got %q", result.State().Status)
	}
}

func TestUseCaseExecuteCompleted(t *testing.T) {
	t.Parallel()

	value := activeTrip(t)
	if err := value.Finish(time.Now()); err != nil {
		t.Fatalf("finish fixture: %v", err)
	}
	fixture := newFixture(t, value)
	fixture.expectTransaction()
	fixture.repository.EXPECT().GetForUpdate(gomock.Any(), value.State().ID).Return(value, nil)

	_, err := fixture.useCase.Execute(context.Background(), value.State().ID)
	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}
}

func TestUseCaseExecuteRepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*fixture, error)
	}{
		{
			name: "get",
			configure: func(f *fixture, wantErr error) {
				f.repository.EXPECT().GetForUpdate(gomock.Any(), f.value.State().ID).Return(domaintrip.Trip{}, wantErr)
			},
		},
		{
			name: "update",
			configure: func(f *fixture, wantErr error) {
				f.repository.EXPECT().GetForUpdate(gomock.Any(), f.value.State().ID).Return(f.value, nil)
				f.repository.EXPECT().Update(gomock.Any(), gomock.Any(), f.now).Return(domaintrip.Trip{}, wantErr)
			},
		},
		{
			name: "history",
			configure: func(f *fixture, wantErr error) {
				f.repository.EXPECT().GetForUpdate(gomock.Any(), f.value.State().ID).Return(f.value, nil)
				f.repository.EXPECT().Update(gomock.Any(), gomock.Any(), f.now).DoAndReturn(
					func(_ context.Context, value domaintrip.Trip, _ time.Time) (domaintrip.Trip, error) {
						return value, nil
					},
				)
				f.repository.EXPECT().AddStatusHistory(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(wantErr)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFixture(t, activeTrip(t))
			fixture.expectTransaction()
			wantErr := errors.New("repository error")
			test.configure(fixture, wantErr)

			_, err := fixture.useCase.Execute(context.Background(), fixture.value.State().ID)
			if !errors.Is(err, wantErr) {
				t.Fatalf("expected repository error, got %v", err)
			}
		})
	}
}

type fixture struct {
	repository *MocktripRepository
	txManager  *MocktxManager
	clock      *Mockclock
	logger     *Mocklogger
	metrics    *MocktripMetrics
	useCase    *tripfinish.UseCase
	value      domaintrip.Trip
	now        time.Time
}

func newFixture(t *testing.T, value domaintrip.Trip) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	repository := NewMocktripRepository(ctrl)
	txManager := NewMocktxManager(ctrl)
	clock := NewMockclock(ctrl)
	logger := NewMocklogger(ctrl)
	metrics := NewMocktripMetrics(ctrl)
	now := time.Date(2026, time.November, 10, 13, 0, 0, 0, time.UTC)
	clock.EXPECT().Now().Return(now)
	return &fixture{
		repository: repository,
		txManager:  txManager,
		clock:      clock,
		logger:     logger,
		metrics:    metrics,
		useCase:    tripfinish.New(repository, txManager, clock, logger, metrics),
		value:      value,
		now:        now,
	}
}

func (f *fixture) expectTransaction() {
	f.txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
}

func activeTrip(t *testing.T) domaintrip.Trip {
	t.Helper()
	value, err := domaintrip.New(uuid.New(), domaintrip.NewTrip{
		UserID: uuid.New(), DriverID: uuid.New(), Price: 100,
	}, time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	return value
}
