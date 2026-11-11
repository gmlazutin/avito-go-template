package get_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/gmlazutin/avito-go-template/internal/http/domainerror"
	tripget "github.com/gmlazutin/avito-go-template/internal/http/trip/get"
)

func TestHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		useCaseErr error
		wantStatus int
		wantCode   string
	}{
		{name: "not found", useCaseErr: domaintrip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: domainerror.CodeTripNotFound},
		{name: "internal", useCaseErr: errors.New("select trip: secret"), wantStatus: http.StatusInternalServerError, wantCode: domainerror.CodeInternalError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			useCase := NewMockuseCase(ctrl)
			id := uuid.New()
			useCase.EXPECT().Execute(gomock.Any(), id).Return(domaintrip.Trip{}, test.useCaseErr)
			recorder := httptest.NewRecorder()
			tripget.NewHandler(useCase).Handle(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil), id)
			assertProblem(t, recorder, test.wantStatus, test.wantCode)
			if strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("internal details leaked: %s", recorder.Body.String())
			}
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		useCase := NewMockuseCase(ctrl)
		value := newTrip(t)
		useCase.EXPECT().Execute(gomock.Any(), value.State().ID).Return(value, nil)
		recorder := httptest.NewRecorder()
		tripget.NewHandler(useCase).Handle(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trips/id", nil), value.State().ID)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})
}

func assertProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("expected status %d, got %d", status, recorder.Code)
	}
	var problem httpv1gen.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != code {
		t.Fatalf("expected code %q, got %q", code, problem.Code)
	}
}

func newTrip(t *testing.T) domaintrip.Trip {
	t.Helper()
	value, err := domaintrip.New(uuid.New(), domaintrip.NewTrip{
		UserID: uuid.New(), DriverID: uuid.New(), Price: 100,
	}, time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	return value
}
