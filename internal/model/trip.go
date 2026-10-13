package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDriverBusy          = errors.New("driver already has an active trip")
	ErrTripNotFound        = errors.New("trip not found")
	ErrTripCompleted       = errors.New("trip already completed")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with request")
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

type CreateTripIdempotency struct {
	RequestHash []byte
	TripID      uuid.UUID
}

type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartPoint     Coordinates
	EndPoint       Coordinates
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	LastPositionAt *time.Time
}
