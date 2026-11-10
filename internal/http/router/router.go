package router

import (
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
	"github.com/go-chi/chi/v5"
)

func New(server *Server) http.Handler {
	router := chi.NewRouter()
	httpv1gen.HandlerWithOptions(server, httpv1gen.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: server.OpenAPIError,
	})
	return router
}
