package create

import (
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	domaintrip "github.com/gmlazutin/avito-go-template/internal/domain/trip"
	"github.com/gmlazutin/avito-go-template/internal/http/domainerror"
	"github.com/gmlazutin/avito-go-template/internal/http/response"
	httptrip "github.com/gmlazutin/avito-go-template/internal/http/trip"
)

type Handler struct {
	useCase useCase
}

func NewHandler(useCase useCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params httpv1gen.CreateTripParams) {
	request, err := decodeRequest(w, r)
	if err != nil {
		domainerror.Write(w, r, domainerror.InvalidRequest(err))
		return
	}
	value, created, err := h.useCase.Execute(r.Context(), domaintrip.NewTrip{
		UserID:     request.UserId,
		DriverID:   request.DriverId,
		StartPoint: domaintrip.Coordinates(request.StartPoint),
		EndPoint:   domaintrip.Coordinates(request.EndPoint),
		Price:      request.Price,
	}, params.IdempotencyKey)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
		w.Header().Set("Location", "/api/v1/trips/"+value.State().ID.String())
	}
	if err := response.JSON(w, status, httptrip.Response(value)); err != nil {
		domainerror.Write(w, r, err)
	}
}
