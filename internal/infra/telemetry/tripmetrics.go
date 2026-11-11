package telemetry

import (
	"context"
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
)

type TripMetrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
	active    atomic.Int64
}

func NewTripMetrics(provider metric.MeterProvider, initialActive int64) (*TripMetrics, error) {
	meter := provider.Meter("trip-service/trip")
	created, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Number of created trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips created metric: %w", err)
	}
	completed, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Number of completed trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips completed metric: %w", err)
	}
	metrics := &TripMetrics{created: created, completed: completed}
	metrics.active.Store(initialActive)
	if _, err := meter.Int64ObservableGauge(
		"tripgo.trips.active",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Number of active trips"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(metrics.active.Load())
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("create active trips metric: %w", err)
	}
	return metrics, nil
}

func (m *TripMetrics) TripCreated(ctx context.Context) {
	m.created.Add(ctx, 1)
	m.active.Add(1)
}

func (m *TripMetrics) TripCompleted(ctx context.Context) {
	m.completed.Add(ctx, 1)
	m.active.Add(-1)
}
