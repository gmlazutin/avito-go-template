package trip

import (
	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
)

func Response(value domaintrip.Trip) httpv1gen.Trip {
	state := value.State()
	return httpv1gen.Trip{
		Id:       state.ID,
		UserId:   state.UserID,
		DriverId: state.DriverID,
		StartPoint: httpv1gen.Coordinates{
			Latitude:  state.StartPoint.Latitude,
			Longitude: state.StartPoint.Longitude,
		},
		EndPoint: httpv1gen.Coordinates{
			Latitude:  state.EndPoint.Latitude,
			Longitude: state.EndPoint.Longitude,
		},
		Price:          state.Price,
		Status:         httpv1gen.TripStatus(state.Status),
		StartedAt:      state.StartedAt,
		FinishedAt:     state.FinishedAt,
		LastPositionAt: state.LastPositionAt,
	}
}
