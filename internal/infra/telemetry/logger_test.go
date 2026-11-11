package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandlerAddsSpanContext(t *testing.T) {
	t.Parallel()

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("parse trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("parse span id: %v", err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	}))
	var output bytes.Buffer
	logger := slog.New(traceHandler{next: slog.NewJSONHandler(&output, nil)})
	logger.InfoContext(ctx, "trip created")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if record["trace_id"] != traceID.String() || record["span_id"] != spanID.String() {
		t.Fatalf("unexpected trace correlation fields: %v", record)
	}
}

func TestTraceHandlerAddsEmptyCorrelationWithoutSpan(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(traceHandler{next: slog.NewJSONHandler(&output, nil)})
	logger.InfoContext(context.Background(), "service started")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if record["trace_id"] != "" || record["span_id"] != "" {
		t.Fatalf("expected empty correlation fields: %v", record)
	}
}
