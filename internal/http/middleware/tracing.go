package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(next http.Handler, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) http.Handler {
	traced := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routeFromRequest(r)
		span := trace.SpanFromContext(r.Context())
		span.SetName(r.Method + " " + route)
		span.SetAttributes(attribute.String("http.route", route))
		next.ServeHTTP(w, r)
	})
	return otelhttp.NewHandler(
		traced,
		"HTTP",
		otelhttp.WithTracerProvider(tracerProvider),
		otelhttp.WithMeterProvider(meterProvider),
		otelhttp.WithPropagators(otel.GetTextMapPropagator()),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + routeFromRequest(r)
		}),
	)
}
