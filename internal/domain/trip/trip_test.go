package trip_test

import (
	"errors"
	"math"
	"testing"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	valid := domaintrip.NewTrip{
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 59.9398, Longitude: 30.3146},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.929, Longitude: 30.3626},
		Price:      1450,
	}

	tests := []struct {
		name      string
		id        uuid.UUID
		input     domaintrip.NewTrip
		startedAt time.Time
		wantField string
	}{
		{name: "empty id", id: uuid.Nil, input: valid, startedAt: now, wantField: "id"},
		{name: "empty user", id: uuid.New(), input: withUserID(valid, uuid.Nil), startedAt: now, wantField: "user_id"},
		{name: "empty driver", id: uuid.New(), input: withDriverID(valid, uuid.Nil), startedAt: now, wantField: "driver_id"},
		{name: "negative price", id: uuid.New(), input: withPrice(valid, -1), startedAt: now, wantField: "price"},
		{name: "empty start time", id: uuid.New(), input: valid, wantField: "started_at"},
		{name: "invalid start latitude", id: uuid.New(), input: withStartLatitude(valid, 91), startedAt: now, wantField: "start_point.latitude"},
		{name: "nan start latitude", id: uuid.New(), input: withStartLatitude(valid, math.NaN()), startedAt: now, wantField: "start_point.latitude"},
		{name: "invalid end longitude", id: uuid.New(), input: withEndLongitude(valid, -181), startedAt: now, wantField: "end_point.longitude"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := domaintrip.New(test.id, test.input, test.startedAt)
			var validationErr *domaintrip.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if validationErr.Field != test.wantField {
				t.Fatalf("expected field %q, got %q", test.wantField, validationErr.Field)
			}
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		value, err := domaintrip.New(id, valid, now)
		if err != nil {
			t.Fatalf("create trip: %v", err)
		}
		state := value.State()
		if state.ID != id || state.Status != domaintrip.StatusActive || state.FinishedAt != nil {
			t.Fatalf("unexpected state: %+v", state)
		}
	})
}

func TestTripFinish(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(15 * time.Minute)
	value, err := domaintrip.New(uuid.New(), domaintrip.NewTrip{
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{},
		EndPoint:   domaintrip.Coordinates{},
		Price:      100,
	}, startedAt)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	if err := value.Finish(finishedAt); err != nil {
		t.Fatalf("finish active trip: %v", err)
	}
	state := value.State()
	if state.Status != domaintrip.StatusCompleted {
		t.Fatalf("expected completed status, got %q", state.Status)
	}
	if state.FinishedAt == nil || !state.FinishedAt.Equal(finishedAt) {
		t.Fatalf("expected finished_at %s, got %v", finishedAt, state.FinishedAt)
	}

	if err := value.Finish(finishedAt.Add(time.Minute)); !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}
}

func TestRestoreRejectsBrokenInvariant(t *testing.T) {
	t.Parallel()

	_, err := domaintrip.Restore(domaintrip.State{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		Status:     domaintrip.StatusCompleted,
		StartedAt:  time.Now(),
		FinishedAt: nil,
	})
	var validationErr *domaintrip.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "finished_at" {
		t.Fatalf("expected finished_at ValidationError, got %v", err)
	}
}

func withUserID(input domaintrip.NewTrip, value uuid.UUID) domaintrip.NewTrip {
	input.UserID = value
	return input
}

func withDriverID(input domaintrip.NewTrip, value uuid.UUID) domaintrip.NewTrip {
	input.DriverID = value
	return input
}

func withPrice(input domaintrip.NewTrip, value int64) domaintrip.NewTrip {
	input.Price = value
	return input
}

func withStartLatitude(input domaintrip.NewTrip, value float64) domaintrip.NewTrip {
	input.StartPoint.Latitude = value
	return input
}

func withEndLongitude(input domaintrip.NewTrip, value float64) domaintrip.NewTrip {
	input.EndPoint.Longitude = value
	return input
}
