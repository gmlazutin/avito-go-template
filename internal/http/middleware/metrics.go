package middleware

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type HTTPMetrics struct {
	activeRequests metric.Int64UpDownCounter
}

func NewHTTPMetrics(provider metric.MeterProvider) (*HTTPMetrics, error) {
	activeRequests, err := provider.Meter("trip-service/http").Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of active HTTP server requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active requests metric: %w", err)
	}
	return &HTTPMetrics{activeRequests: activeRequests}, nil
}

func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attributes := []attribute.KeyValue{
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", routeFromRequest(r)),
		}
		options := metric.WithAttributes(attributes...)
		m.activeRequests.Add(r.Context(), 1, options)
		defer m.activeRequests.Add(context.WithoutCancel(r.Context()), -1, options)
		if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
			labeler.Add(attribute.String("http.route", routeFromRequest(r)))
		}
		next.ServeHTTP(w, r)
	})
}
