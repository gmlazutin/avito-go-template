package domainerror

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
)

const (
	CodeInvalidRequest      = "invalid_request"
	CodeTripNotFound        = "trip_not_found"
	CodeTripCompleted       = "trip_completed"
	CodeDriverBusy          = "driver_busy"
	CodeIdempotencyConflict = "idempotency_conflict"
	CodeInternalError       = "internal_error"
)

type InvalidRequestError struct {
	err error
}

func InvalidRequest(err error) error {
	return &InvalidRequestError{err: err}
}

func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid request: %v", e.err)
}

func (e *InvalidRequestError) Unwrap() error {
	return e.err
}

func Problem(err error) (int, httpv1gen.Problem) {
	var validationErr *domaintrip.ValidationError
	var invalidRequestErr *InvalidRequestError
	switch {
	case errors.As(err, &validationErr):
		return newProblem(
			http.StatusBadRequest,
			CodeInvalidRequest,
			"Invalid request",
			fmt.Sprintf("%s: %s", validationErr.Field, validationErr.Reason),
			"invalid-request",
		)
	case errors.As(err, &invalidRequestErr):
		return newProblem(
			http.StatusBadRequest,
			CodeInvalidRequest,
			"Invalid request",
			"Request validation failed",
			"invalid-request",
		)
	case errors.Is(err, domaintrip.ErrTripNotFound):
		return newProblem(
			http.StatusNotFound,
			CodeTripNotFound,
			"Trip not found",
			"Trip was not found",
			"trip-not-found",
		)
	case errors.Is(err, domaintrip.ErrTripCompleted):
		return newProblem(
			http.StatusConflict,
			CodeTripCompleted,
			"Trip completed",
			"Operation is not allowed for a completed trip",
			"trip-completed",
		)
	case errors.Is(err, domaintrip.ErrDriverBusy):
		return newProblem(
			http.StatusConflict,
			CodeDriverBusy,
			"Driver busy",
			"Driver already has an active trip",
			"driver-busy",
		)
	case errors.Is(err, domaintrip.ErrIdempotencyConflict):
		return newProblem(
			http.StatusConflict,
			CodeIdempotencyConflict,
			"Idempotency conflict",
			"Idempotency-Key was already used with a different request body",
			"idempotency-conflict",
		)
	default:
		return newProblem(
			http.StatusInternalServerError,
			CodeInternalError,
			"Internal Server Error",
			"Internal server error",
			"internal-error",
		)
	}
}

func newProblem(status int, code, title, detail, kind string) (int, httpv1gen.Problem) {
	return status, httpv1gen.Problem{
		Type:   "https://tripgo.example/problems/" + kind,
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}
