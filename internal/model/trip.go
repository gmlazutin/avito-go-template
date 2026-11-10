package model

import (
	"github.com/google/uuid"
)

type CreateTripIdempotency struct {
	RequestHash []byte
	TripID      uuid.UUID
}
