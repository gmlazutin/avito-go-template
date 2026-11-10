package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestChainCorrelatesRouteLogTraceAndMetrics(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/items/{itemID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler, output, spans, reader := newTestChain(t, router)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/items/42", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	logRecord := decodeLastLog(t, output)
	if logRecord["route"] != "/items/{itemID}" || logRecord["status"] != float64(http.StatusNoContent) {
		t.Fatalf("unexpected request log: %v", logRecord)
	}

	finished := spans.GetSpans()
	if len(finished) != 1 {
		t.Fatalf("expected one HTTP span, got %d", len(finished))
	}
	if finished[0].Name != "GET /items/{itemID}" {
		t.Fatalf("unexpected span name %q", finished[0].Name)
	}
	if value := spanAttribute(finished[0].Attributes, "http.route"); value != "/items/{itemID}" {
		t.Fatalf("unexpected http.route attribute %q", value)
	}

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	names := metricNames(metrics)
	for _, name := range []string{"http.server.request.duration", "http.server.active_requests"} {
		if !names[name] {
			t.Fatalf("metric %q was not recorded; got %v", name, names)
		}
	}
	durationAttributes := histogramAttributes(metrics, "http.server.request.duration")
	assertMetricAttribute(t, durationAttributes, "http.request.method", http.MethodGet)
	assertMetricAttribute(t, durationAttributes, "http.route", "/items/{itemID}")
	assertMetricAttribute(t, durationAttributes, "http.response.status_code", int64(http.StatusNoContent))
}

func TestChainRecoversPanic(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/panic/{itemID}", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	handler, output, spans, _ := newTestChain(t, router)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic/42", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
	var problem httpv1gen.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != "internal_error" {
		t.Fatalf("expected internal_error, got %q", problem.Code)
	}
	logRecord := decodeLastLog(t, output)
	if logRecord["level"] != "ERROR" || logRecord["route"] != "/panic/{itemID}" {
		t.Fatalf("unexpected panic log: %v", logRecord)
	}
	if stack, ok := logRecord["stack"].(string); !ok || !strings.Contains(stack, "panic") {
		t.Fatalf("panic stack is missing: %v", logRecord)
	}

	finished := spans.GetSpans()
	if len(finished) != 1 || finished[0].Status.Code != codes.Error {
		t.Fatalf("panic span was not marked as error: %+v", finished)
	}
	if value := spanAttribute(finished[0].Attributes, "error.type"); value != "internal_error" {
		t.Fatalf("unexpected error.type %q", value)
	}
}

func newTestChain(
	t *testing.T,
	router chi.Router,
) (http.Handler, *bytes.Buffer, *tracetest.InMemoryExporter, *sdkmetric.ManualReader) {
	t.Helper()
	spanExporter := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExporter))
	t.Cleanup(func() {
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})
	metrics, err := NewHTTPMetrics(meterProvider)
	if err != nil {
		t.Fatalf("create HTTP metrics: %v", err)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	return Chain(router, router, logger, tracerProvider, meterProvider, metrics), &output, spanExporter, reader
}

func decodeLastLog(t *testing.T, output *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &record); err != nil {
		t.Fatalf("decode request log: %v", err)
	}
	return record
}

func spanAttribute(attributes []attribute.KeyValue, key string) string {
	for _, item := range attributes {
		if string(item.Key) == key {
			return item.Value.AsString()
		}
	}
	return ""
}

func metricNames(metrics metricdata.ResourceMetrics) map[string]bool {
	names := make(map[string]bool)
	for _, scope := range metrics.ScopeMetrics {
		for _, item := range scope.Metrics {
			names[item.Name] = true
		}
	}
	return names
}

func histogramAttributes(metrics metricdata.ResourceMetrics, name string) attribute.Set {
	for _, scope := range metrics.ScopeMetrics {
		for _, item := range scope.Metrics {
			if item.Name != name {
				continue
			}
			if histogram, ok := item.Data.(metricdata.Histogram[float64]); ok && len(histogram.DataPoints) != 0 {
				return histogram.DataPoints[0].Attributes
			}
		}
	}
	return attribute.NewSet()
}

func assertMetricAttribute(t *testing.T, attributes attribute.Set, key string, want any) {
	t.Helper()
	value, ok := attributes.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("metric attribute %q is missing from %v", key, attributes)
	}
	if got := value.AsInterface(); got != want {
		t.Fatalf("expected metric attribute %s=%v, got %v", key, want, got)
	}
}
