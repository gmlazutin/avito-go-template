package trip

import (
	"errors"
	"fmt"
)

var (
	ErrTripNotFound        = errors.New("trip not found")
	ErrTripCompleted       = errors.New("trip already completed")
	ErrDriverBusy          = errors.New("driver already has an active trip")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with request")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validate %s: %s", e.Field, e.Reason)
}
