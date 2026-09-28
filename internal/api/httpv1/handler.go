package httpv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	"github.com/gmlazutin/avito-go-template/internal/apierror"
	"github.com/gmlazutin/avito-go-template/internal/app/log"
	"github.com/gmlazutin/avito-go-template/internal/model"
	"github.com/gmlazutin/avito-go-template/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxRequestBodySize = 1 << 20

type Handler struct {
	service      *service.Service
	database     *pgxpool.Pool
	queryTimeout time.Duration
	logger       *slog.Logger
}

func New(service *service.Service, database *pgxpool.Pool, queryTimeout time.Duration, logger *slog.Logger) *Handler {
	return &Handler{
		service:      service,
		database:     database,
		queryTimeout: queryTimeout,
		logger:       logger,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ httpv1gen.CreateTripParams) {
	var request httpv1gen.CreateTripJSONRequestBody
	if err := decodeCreateTrip(w, r, &request); err != nil {
		h.writeError(w, r, apierror.Wrap(apierror.CodeInvalidRequest, err))
		return
	}
	if request.UserId == uuid.Nil || request.DriverId == uuid.Nil || request.Price < 0 ||
		!validCoordinates(request.StartPoint) || !validCoordinates(request.EndPoint) {
		h.writeError(w, r, apierror.New(apierror.CodeInvalidRequest))
		return
	}

	trip, err := h.service.Create(r.Context(), model.NewTrip{
		UserID:   request.UserId,
		DriverID: request.DriverId,
		StartPoint: model.Coordinates{
			Latitude:  request.StartPoint.Latitude,
			Longitude: request.StartPoint.Longitude,
		},
		EndPoint: model.Coordinates{
			Latitude:  request.EndPoint.Latitude,
			Longitude: request.EndPoint.Longitude,
		},
		Price: request.Price,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	writeJSON(w, http.StatusCreated, tripResponse(trip))
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID httpv1gen.TripId) {
	trip, err := h.service.Get(r.Context(), tripID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tripResponse(trip))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID httpv1gen.TripId) {
	trip, err := h.service.Finish(r.Context(), tripID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tripResponse(trip))
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, httpv1gen.HealthResponse{
		Status: httpv1gen.Ok,
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.database.Ping(ctx); err != nil {
		h.logger.Warn("readiness check failed", log.Error(err))
		writeJSON(w, http.StatusServiceUnavailable, httpv1gen.HealthResponse{
			Status: httpv1gen.Unavailable,
		})
		return
	}
	writeJSON(w, http.StatusOK, httpv1gen.HealthResponse{
		Status: httpv1gen.Ok,
	})
}

func (h *Handler) OpenAPIError(w http.ResponseWriter, r *http.Request, err error) {
	h.writeError(w, r, apierror.Wrap(apierror.CodeInvalidRequest, err))
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	publicErr := asAPIError(err)
	info := publicErr.Info()
	status := info.Codes.HTTPv1
	attributes := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"code", info.Code,
		log.Error(err),
	}
	if status >= http.StatusInternalServerError {
		h.logger.Error("request failed", attributes...)
	} else {
		h.logger.Debug("request failed", attributes...)
	}
	h.problem(w, r, publicErr)
}

func asAPIError(err error) *apierror.Error {
	var publicErr *apierror.Error
	if errors.As(err, &publicErr) {
		return publicErr
	}
	return apierror.Wrap(apierror.CodeInternalError, err)
}

func (h *Handler) problem(w http.ResponseWriter, r *http.Request, publicErr *apierror.Error) {
	info := publicErr.Info()
	instance := r.URL.Path
	detail := info.Message
	w.Header().Set("Content-Type", "application/problem+json")
	writeJSONStatus(w, info.Codes.HTTPv1, httpv1gen.Problem{
		Type:     info.Type,
		Title:    info.Title,
		Status:   int32(info.Codes.HTTPv1),
		Detail:   &detail,
		Instance: &instance,
		Code:     info.Code,
	})
}

func decodeCreateTrip(w http.ResponseWriter, r *http.Request, target *httpv1gen.CreateTripJSONRequestBody) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.New("request body is invalid or exceeds size limit")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return errors.New("request body is invalid")
	}
	for _, name := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("request body is missing required fields")
		}
	}

	for _, name := range []string{"start_point", "end_point"} {
		var coordinates map[string]json.RawMessage
		if err := json.Unmarshal(fields[name], &coordinates); err != nil {
			return errors.New("request coordinates are invalid")
		}
		for _, coordinate := range []string{"latitude", "longitude"} {
			value, ok := coordinates[coordinate]
			if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return errors.New("request coordinates are missing required fields")
			}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("request body is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func validCoordinates(value httpv1gen.Coordinates) bool {
	return value.Latitude >= -90 && value.Latitude <= 90 && value.Longitude >= -180 && value.Longitude <= 180
}

func tripResponse(trip model.Trip) httpv1gen.Trip {
	return httpv1gen.Trip{
		Id:       trip.ID,
		UserId:   trip.UserID,
		DriverId: trip.DriverID,
		StartPoint: httpv1gen.Coordinates{
			Latitude:  trip.StartPoint.Latitude,
			Longitude: trip.StartPoint.Longitude,
		},
		EndPoint: httpv1gen.Coordinates{
			Latitude:  trip.EndPoint.Latitude,
			Longitude: trip.EndPoint.Longitude,
		},
		Price:          trip.Price,
		Status:         httpv1gen.TripStatus(trip.Status),
		StartedAt:      trip.StartedAt,
		FinishedAt:     trip.FinishedAt,
		LastPositionAt: trip.LastPositionAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
