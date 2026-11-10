package domainerror

import (
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/http/response"
)

type errorRecorder interface {
	RecordError(error)
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	status, body := Problem(err)
	instance := r.URL.Path
	body.Instance = &instance
	if recorder, ok := w.(errorRecorder); ok {
		recorder.RecordError(err)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	if writeErr := response.JSON(w, status, body); writeErr != nil {
		if recorder, ok := w.(errorRecorder); ok {
			recorder.RecordError(writeErr)
		}
	}
}
