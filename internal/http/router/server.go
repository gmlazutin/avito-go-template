package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	"github.com/gmlazutin/avito-go-template/internal/http/domainerror"
	"github.com/gmlazutin/avito-go-template/internal/http/response"
	tripcreate "github.com/gmlazutin/avito-go-template/internal/http/trip/create"
	tripfinish "github.com/gmlazutin/avito-go-template/internal/http/trip/finish"
	tripget "github.com/gmlazutin/avito-go-template/internal/http/trip/get"
)

type healthChecker interface {
	Ping(context.Context) error
}

type Server struct {
	createTrip   *tripcreate.Handler
	getTrip      *tripget.Handler
	finishTrip   *tripfinish.Handler
	health       healthChecker
	queryTimeout time.Duration
}

func NewServer(
	createTrip *tripcreate.Handler,
	getTrip *tripget.Handler,
	finishTrip *tripfinish.Handler,
	health healthChecker,
	queryTimeout time.Duration,
) *Server {
	return &Server{
		createTrip:   createTrip,
		getTrip:      getTrip,
		finishTrip:   finishTrip,
		health:       health,
		queryTimeout: queryTimeout,
	}
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params httpv1gen.CreateTripParams) {
	s.createTrip.Handle(w, r, params)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, id httpv1gen.TripId) {
	s.getTrip.Handle(w, r, id)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, id httpv1gen.TripId) {
	s.finishTrip.Handle(w, r, id)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	_ = response.JSON(w, http.StatusOK, httpv1gen.HealthResponse{Status: httpv1gen.Ok})
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.queryTimeout)
	defer cancel()
	if err := s.health.Ping(ctx); err != nil {
		_ = response.JSON(w, http.StatusServiceUnavailable, httpv1gen.HealthResponse{Status: httpv1gen.Unavailable})
		return
	}
	_ = response.JSON(w, http.StatusOK, httpv1gen.HealthResponse{Status: httpv1gen.Ok})
}

func (s *Server) OpenAPIError(w http.ResponseWriter, r *http.Request, err error) {
	domainerror.Write(w, r, domainerror.InvalidRequest(err))
}
