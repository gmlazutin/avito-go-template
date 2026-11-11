package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const (
	ProtocolHTTPProtobuf = "http/protobuf"
	ProtocolGRPC         = "grpc"
)

type Config struct {
	ServiceName string
	Version     string
	Protocol    string
	Sampler     string
}

type Providers struct {
	tracer *sdktrace.TracerProvider
	meter  *metric.MeterProvider
}

func NewProviders(ctx context.Context, cfg Config) (*Providers, error) {
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	traceExporter, metricExporter, err := newExporters(ctx, cfg.Protocol)
	if err != nil {
		return nil, err
	}
	sampler, err := newSampler(cfg.Sampler)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		_ = metricExporter.Shutdown(ctx)
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Providers{tracer: tracerProvider, meter: meterProvider}, nil
}

func (p *Providers) TracerProvider() *sdktrace.TracerProvider {
	return p.tracer
}

func (p *Providers) MeterProvider() *metric.MeterProvider {
	return p.meter
}

func (p *Providers) Shutdown(ctx context.Context) error {
	return errors.Join(p.meter.Shutdown(ctx), p.tracer.Shutdown(ctx))
}

func newExporters(ctx context.Context, protocol string) (sdktrace.SpanExporter, metric.Exporter, error) {
	switch protocol {
	case ProtocolHTTPProtobuf:
		traceExporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("create OTLP HTTP trace exporter: %w", err)
		}
		metricExporter, err := otlpmetrichttp.New(ctx)
		if err != nil {
			_ = traceExporter.Shutdown(ctx)
			return nil, nil, fmt.Errorf("create OTLP HTTP metric exporter: %w", err)
		}
		return traceExporter, metricExporter, nil
	case ProtocolGRPC:
		traceExporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("create OTLP gRPC trace exporter: %w", err)
		}
		metricExporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			_ = traceExporter.Shutdown(ctx)
			return nil, nil, fmt.Errorf("create OTLP gRPC metric exporter: %w", err)
		}
		return traceExporter, metricExporter, nil
	default:
		return nil, nil, fmt.Errorf("unsupported OTEL_EXPORTER_OTLP_PROTOCOL %q", protocol)
	}
}

func newSampler(value string) (sdktrace.Sampler, error) {
	switch value {
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample()), nil
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_SAMPLER %q", value)
	}
}
