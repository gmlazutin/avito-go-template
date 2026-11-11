package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

func NewLogger(service string, level slog.Level) *slog.Logger {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(traceHandler{next: base}).With("service", service)
}

func Error(err error) slog.Attr {
	return slog.String("error", err.Error())
}

type traceHandler struct {
	next slog.Handler
}

func (h traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	spanContext := trace.SpanContextFromContext(ctx)
	traceID := ""
	spanID := ""
	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
		spanID = spanContext.SpanID().String()
	}
	record.AddAttrs(slog.String("trace_id", traceID), slog.String("span_id", spanID))
	return h.next.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{next: h.next.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{next: h.next.WithGroup(name)}
}
