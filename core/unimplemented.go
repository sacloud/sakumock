package core

import (
	"net/http"
	"strings"
)

// UnimplementedKind is the Route.Kind of the routes UnimplementedRoutes returns.
const UnimplementedKind = "unimplemented"

// UnimplementedRoutes registers the spec operations a service does not implement
// (the generated unimplementedRoutes, "METHOD /path" keys) so they answer
// 501 Not Implemented in the service's error envelope instead of a misleading
// 404. Append them to routeTable() before the inspection routes; they bypass
// fault injection, rate limiting and validation since there is no behavior to
// emulate.
func UnimplementedRoutes(keys []string, writeErr BodyValidationErrorWriter) []RegisteredRoute {
	routes := make([]RegisteredRoute, 0, len(keys))
	for _, key := range keys {
		method, path, ok := strings.Cut(key, " ")
		if !ok {
			panic("core.UnimplementedRoutes: malformed route key " + key)
		}
		msg := key + " is not implemented by sakumock"
		routes = append(routes, RegisteredRoute{
			Route: Route{Method: method, Path: path, Description: "Not implemented (501)", Kind: UnimplementedKind},
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				writeErr(w, http.StatusNotImplemented, msg)
			},
		})
	}
	return routes
}
