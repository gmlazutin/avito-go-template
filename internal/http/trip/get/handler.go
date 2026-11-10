package get

import (
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
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

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id httpv1gen.TripId) {
	value, err := h.useCase.Execute(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}
	if err := response.JSON(w, http.StatusOK, httptrip.Response(value)); err != nil {
		domainerror.Write(w, r, err)
	}
}
