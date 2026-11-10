package domainerror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/gmlazutin/avito-go-template/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "invalid request",
			err:        domainerror.InvalidRequest(errors.New("broken json")),
			wantStatus: http.StatusBadRequest,
			wantCode:   domainerror.CodeInvalidRequest,
			wantDetail: "Request validation failed",
		},
		{
			name:       "validation",
			err:        &domaintrip.ValidationError{Field: "price", Reason: "must not be negative"},
			wantStatus: http.StatusBadRequest,
			wantCode:   domainerror.CodeInvalidRequest,
			wantDetail: "price: must not be negative",
		},
		{name: "not found", err: domaintrip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: domainerror.CodeTripNotFound, wantDetail: "Trip was not found"},
		{name: "completed", err: domaintrip.ErrTripCompleted, wantStatus: http.StatusConflict, wantCode: domainerror.CodeTripCompleted, wantDetail: "Operation is not allowed for a completed trip"},
		{name: "driver busy", err: domaintrip.ErrDriverBusy, wantStatus: http.StatusConflict, wantCode: domainerror.CodeDriverBusy, wantDetail: "Driver already has an active trip"},
		{name: "idempotency", err: domaintrip.ErrIdempotencyConflict, wantStatus: http.StatusConflict, wantCode: domainerror.CodeIdempotencyConflict, wantDetail: "Idempotency-Key was already used with a different request body"},
		{name: "internal", err: errors.New("password=secret"), wantStatus: http.StatusInternalServerError, wantCode: domainerror.CodeInternalError, wantDetail: "Internal server error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			status, problem := domainerror.Problem(fmt.Errorf("operation failed: %w", test.err))
			if status != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, status)
			}
			if problem.Code != test.wantCode {
				t.Fatalf("expected code %q, got %q", test.wantCode, problem.Code)
			}
			if problem.Detail == nil || *problem.Detail != test.wantDetail {
				t.Fatalf("expected detail %q, got %v", test.wantDetail, problem.Detail)
			}
		})
	}
}
