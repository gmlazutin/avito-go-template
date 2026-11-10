package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/gmlazutin/avito-go-template/internal/http/domainerror"
)

func Logging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := recorderFrom(w)
		defer func() {
			recovered := recover()
			if recovered != nil && recorder != nil {
				recorder.recordPanic(fmt.Errorf("panic recovered: %v", recovered), debug.Stack())
			}
			logRequest(r, recorder, logger, time.Since(startedAt))
			if recovered != nil {
				panic(recovered)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func logRequest(r *http.Request, recorder *responseRecorder, logger *slog.Logger, duration time.Duration) {
	status := http.StatusOK
	var requestErr error
	var stack []byte
	if recorder != nil {
		status, stack, _, requestErr = recorder.snapshot()
	}
	attributes := []any{
		"method", r.Method,
		"route", routeFromRequest(r),
		"status", status,
		"duration_ms", duration.Milliseconds(),
	}
	if requestErr != nil {
		_, problem := domainerror.Problem(requestErr)
		span := trace.SpanFromContext(r.Context())
		span.SetStatus(codes.Error, problem.Code)
		span.SetAttributes(attribute.String("error.type", problem.Code))
		if status >= http.StatusInternalServerError {
			attributes = append(attributes, "error", requestErr.Error())
		}
	}
	if len(stack) != 0 {
		attributes = append(attributes, "stack", string(stack))
	}
	if status >= http.StatusInternalServerError {
		logger.ErrorContext(r.Context(), "request handled", attributes...)
		return
	}
	logger.InfoContext(r.Context(), "request handled", attributes...)
}
