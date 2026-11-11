package middleware

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func Chain(
	next http.Handler,
	routes chi.Routes,
	logger *slog.Logger,
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
	metrics *HTTPMetrics,
) http.Handler {
	handler := metrics.Middleware(next)
	handler = Logging(handler, logger)
	handler = Tracing(handler, tracerProvider, meterProvider)
	handler = Recover(handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := resolveRoute(routes, r)
		recorder := newResponseRecorder(w)
		handler.ServeHTTP(recorder, withRoute(r, route))
	})
}
