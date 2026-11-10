package trip

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type NewTrip struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint Coordinates
	EndPoint   Coordinates
	Price      int64
}

type State struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartPoint     Coordinates
	EndPoint       Coordinates
	Price          int64
	Status         Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	LastPositionAt *time.Time
}

type Trip struct {
	state State
}

func New(id uuid.UUID, input NewTrip, startedAt time.Time) (Trip, error) {
	state := State{
		ID:         id,
		UserID:     input.UserID,
		DriverID:   input.DriverID,
		StartPoint: input.StartPoint,
		EndPoint:   input.EndPoint,
		Price:      input.Price,
		Status:     StatusActive,
		StartedAt:  startedAt,
	}
	if err := validateState(state); err != nil {
		return Trip{}, err
	}
	return Trip{state: state}, nil
}

func Restore(state State) (Trip, error) {
	state.FinishedAt = cloneTime(state.FinishedAt)
	state.LastPositionAt = cloneTime(state.LastPositionAt)
	if err := validateState(state); err != nil {
		return Trip{}, fmt.Errorf("restore trip: %w", err)
	}
	return Trip{state: state}, nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.state.Status != StatusActive {
		return ErrTripCompleted
	}
	finishedAt := at
	t.state.Status = StatusCompleted
	t.state.FinishedAt = &finishedAt
	return nil
}

func (t Trip) State() State {
	state := t.state
	state.FinishedAt = cloneTime(state.FinishedAt)
	state.LastPositionAt = cloneTime(state.LastPositionAt)
	return state
}

func validateState(state State) error {
	switch {
	case state.ID == uuid.Nil:
		return validationError("id", "must not be empty")
	case state.UserID == uuid.Nil:
		return validationError("user_id", "must not be empty")
	case state.DriverID == uuid.Nil:
		return validationError("driver_id", "must not be empty")
	case state.Price < 0:
		return validationError("price", "must not be negative")
	case state.StartedAt.IsZero():
		return validationError("started_at", "must not be empty")
	}
	if err := validateCoordinates("start_point", state.StartPoint); err != nil {
		return err
	}
	if err := validateCoordinates("end_point", state.EndPoint); err != nil {
		return err
	}
	switch state.Status {
	case StatusActive:
		if state.FinishedAt != nil {
			return validationError("finished_at", "must be empty for an active trip")
		}
	case StatusCompleted:
		if state.FinishedAt == nil {
			return validationError("finished_at", "must be set for a completed trip")
		}
	default:
		return validationError("status", "must be active or completed")
	}
	return nil
}

func validateCoordinates(field string, coordinates Coordinates) error {
	if math.IsNaN(coordinates.Latitude) || math.IsInf(coordinates.Latitude, 0) ||
		coordinates.Latitude < -90 || coordinates.Latitude > 90 {
		return validationError(field+".latitude", "must be between -90 and 90")
	}
	if math.IsNaN(coordinates.Longitude) || math.IsInf(coordinates.Longitude, 0) ||
		coordinates.Longitude < -180 || coordinates.Longitude > 180 {
		return validationError(field+".longitude", "must be between -180 and 180")
	}
	return nil
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
