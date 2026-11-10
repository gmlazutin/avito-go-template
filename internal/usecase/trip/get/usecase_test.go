package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	tripget "github.com/gmlazutin/avito-go-template/internal/usecase/trip/get"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestUseCaseExecute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repoErr error
	}{
		{name: "not found", repoErr: domaintrip.ErrTripNotFound},
		{name: "repository error", repoErr: errors.New("database unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			id := uuid.New()
			repository.EXPECT().Get(gomock.Any(), id).Return(domaintrip.Trip{}, test.repoErr)

			_, err := tripget.New(repository).Execute(context.Background(), id)
			if !errors.Is(err, test.repoErr) {
				t.Fatalf("expected %v, got %v", test.repoErr, err)
			}
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repository := NewMocktripRepository(ctrl)
		id := uuid.New()
		value, err := domaintrip.New(id, domaintrip.NewTrip{
			UserID: uuid.New(), DriverID: uuid.New(), Price: 100,
		}, time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("create fixture: %v", err)
		}
		repository.EXPECT().Get(gomock.Any(), id).Return(value, nil)

		result, err := tripget.New(repository).Execute(context.Background(), id)
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		if result.State().ID != id {
			t.Fatalf("expected trip %s, got %s", id, result.State().ID)
		}
	})
}
