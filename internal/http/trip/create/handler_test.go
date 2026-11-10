package create_test

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
	tripcreate "github.com/gmlazutin/avito-go-template/internal/http/trip/create"
)

const contentTypeJSON = "application/json"

const validBody = `{
  "user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
  "driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",
  "start_point":{"latitude":59.9398,"longitude":30.3146},
  "end_point":{"latitude":59.929,"longitude":30.3626},
  "price":1450
}`

func TestHandlerSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	useCase := NewMockuseCase(ctrl)
	value := mustTrip(t)
	useCase.EXPECT().Execute(gomock.Any(), gomock.Any(), nil).Return(value, true, nil)
	recorder := performRequest(tripcreate.NewHandler(useCase), validBody, contentTypeJSON)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}
	if recorder.Header().Get("Location") != "/api/v1/trips/"+value.State().ID.String() {
		t.Fatalf("unexpected Location: %q", recorder.Header().Get("Location"))
	}
}

func TestHandlerErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		content    string
		useCaseErr error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid body", body: `{`, content: contentTypeJSON, wantStatus: http.StatusBadRequest, wantCode: domainerror.CodeInvalidRequest},
		{name: "missing content type", body: validBody, wantStatus: http.StatusBadRequest, wantCode: domainerror.CodeInvalidRequest},
		{name: "driver busy", body: validBody, content: contentTypeJSON, useCaseErr: domaintrip.ErrDriverBusy, wantStatus: http.StatusConflict, wantCode: domainerror.CodeDriverBusy},
		{name: "internal", body: validBody, content: contentTypeJSON, useCaseErr: errors.New("database password=secret"), wantStatus: http.StatusInternalServerError, wantCode: domainerror.CodeInternalError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			useCase := NewMockuseCase(ctrl)
			if test.useCaseErr != nil {
				useCase.EXPECT().Execute(gomock.Any(), gomock.Any(), nil).Return(domaintrip.Trip{}, false, test.useCaseErr)
			}
			recorder := performRequest(tripcreate.NewHandler(useCase), test.body, test.content)
			if recorder.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, recorder.Code)
			}
			var problem httpv1gen.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Code != test.wantCode {
				t.Fatalf("expected code %q, got %q", test.wantCode, problem.Code)
			}
			if strings.Contains(recorder.Body.String(), "password") {
				t.Fatalf("internal details leaked: %s", recorder.Body.String())
			}
		})
	}
}

func performRequest(handler *tripcreate.Handler, body, contentType string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	handler.Handle(recorder, request, httpv1gen.CreateTripParams{})
	return recorder
}

func mustTrip(t *testing.T) domaintrip.Trip {
	t.Helper()
	value, err := domaintrip.New(uuid.New(), domaintrip.NewTrip{
		UserID: uuid.New(), DriverID: uuid.New(), Price: 1450,
	}, time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	return value
}
