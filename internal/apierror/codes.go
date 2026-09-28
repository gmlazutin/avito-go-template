package apierror

import "net/http"

const (
	CodeInvalidRequest = "invalid_request"
	CodeTripNotFound   = "trip_not_found"
	CodeTripCompleted  = "trip_completed"
	CodeDriverBusy     = "driver_busy"
	CodeInternalError  = "internal_error"
)

var infos = map[string]Info{
	CodeInvalidRequest: {
		Type:    "https://tripgo.example/problems/invalid-request",
		Title:   "Invalid request",
		Message: "Request validation failed",
		Codes: APICodes{
			HTTPv1: http.StatusBadRequest,
		},
	},
	CodeTripNotFound: {
		Type:    "https://tripgo.example/problems/trip-not-found",
		Title:   "Trip not found",
		Message: "Trip was not found",
		Codes: APICodes{
			HTTPv1: http.StatusNotFound,
		},
	},
	CodeTripCompleted: {
		Type:    "https://tripgo.example/problems/trip-completed",
		Title:   "Trip completed",
		Message: "Operation is not allowed for a completed trip",
		Codes: APICodes{
			HTTPv1: http.StatusConflict,
		},
	},
	CodeDriverBusy: {
		Type:    "https://tripgo.example/problems/driver-busy",
		Title:   "Driver busy",
		Message: "Driver already has an active trip",
		Codes: APICodes{
			HTTPv1: http.StatusConflict,
		},
	},
	CodeInternalError: {
		Type:    "https://tripgo.example/problems/internal-error",
		Title:   "Internal Server Error",
		Message: "Internal server error",
		Codes: APICodes{
			HTTPv1: http.StatusInternalServerError,
		},
	},
}
