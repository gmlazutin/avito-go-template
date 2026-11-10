package telemetry

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestTripMetrics(t *testing.T) {
	t.Parallel()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})
	metrics, err := NewTripMetrics(provider, 2)
	if err != nil {
		t.Fatalf("create trip metrics: %v", err)
	}
	metrics.TripCreated(context.Background())
	metrics.TripCompleted(context.Background())

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	values := int64MetricValues(collected)
	if values["tripgo.trips.created"] != 1 {
		t.Fatalf("expected one created trip, got %d", values["tripgo.trips.created"])
	}
	if values["tripgo.trips.completed"] != 1 {
		t.Fatalf("expected one completed trip, got %d", values["tripgo.trips.completed"])
	}
	if values["tripgo.trips.active"] != 2 {
		t.Fatalf("expected two active trips, got %d", values["tripgo.trips.active"])
	}
}

func int64MetricValues(metrics metricdata.ResourceMetrics) map[string]int64 {
	values := make(map[string]int64)
	for _, scope := range metrics.ScopeMetrics {
		for _, item := range scope.Metrics {
			switch data := item.Data.(type) {
			case metricdata.Sum[int64]:
				for _, point := range data.DataPoints {
					values[item.Name] += point.Value
				}
			case metricdata.Gauge[int64]:
				for _, point := range data.DataPoints {
					values[item.Name] = point.Value
				}
			}
		}
	}
	return values
}
