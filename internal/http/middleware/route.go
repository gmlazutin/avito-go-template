package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const unmatchedRoute = "unmatched"

type routeContextKey struct{}

func withRoute(r *http.Request, route string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), routeContextKey{}, route))
}

func routeFromRequest(r *http.Request) string {
	if route, ok := r.Context().Value(routeContextKey{}).(string); ok && route != "" {
		return route
	}
	return unmatchedRoute
}

func resolveRoute(routes chi.Routes, r *http.Request) string {
	routeContext := chi.NewRouteContext()
	if routes.Match(routeContext, r.Method, r.URL.Path) {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return unmatchedRoute
}
