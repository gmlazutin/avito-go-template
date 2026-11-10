package router

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	httpmiddleware "github.com/gmlazutin/avito-go-template/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func New(
	server *Server,
	logger *slog.Logger,
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
) (http.Handler, error) {
	router := chi.NewRouter()
	httpv1gen.HandlerWithOptions(server, httpv1gen.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: server.OpenAPIError,
	})
	metrics, err := httpmiddleware.NewHTTPMetrics(meterProvider)
	if err != nil {
		return nil, fmt.Errorf("create HTTP metrics: %w", err)
	}
	return httpmiddleware.Chain(router, router, logger, tracerProvider, meterProvider, metrics), nil
}
