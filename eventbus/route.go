package eventbus

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	route := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{
			Route: core.Route{Method: method, Path: path, Description: desc, Kind: "api"},
			// Rate limit outermost, then spec-derived body validation, then
			// the handler.
			Handler: rl(s.validator.Middleware(method, path, h)),
		}
	}
	return []core.RegisteredRoute{
		route("POST", "/commonserviceitem", "Create a process configuration, schedule, or trigger", s.handleCreateItem),
		route("GET", "/commonserviceitem", "List process configurations, schedules, or triggers", s.handleListItems),
		route("GET", "/commonserviceitem/{id}", "Get a process configuration, schedule, or trigger", s.handleGetItem),
		route("PUT", "/commonserviceitem/{id}", "Update a process configuration, schedule, or trigger", s.handleUpdateItem),
		route("DELETE", "/commonserviceitem/{id}", "Delete a process configuration, schedule, or trigger", s.handleDeleteItem),
		route("PUT", "/commonserviceitem/{id}/eventbus/processconfiguration/set-secret", "Set the secret of a process configuration", s.handleSetSecret),

		// Mock-only data-plane endpoints (not rate-limited, like other inspection helpers).
		{Route: core.Route{Method: "POST", Path: "/_sakumock/events", Description: "Inject an event and fire matching triggers", Kind: "inspection"}, Handler: s.handleInjectEvent},
		{Route: core.Route{Method: "POST", Path: "/_sakumock/tick", Description: "Evaluate schedules and fire those due (optional ?at=<RFC3339|epoch-seconds>)", Kind: "inspection"}, Handler: s.handleTick},
		{Route: core.Route{Method: "GET", Path: "/_sakumock/deliveries", Description: "List recorded firings", Kind: "inspection"}, Handler: s.handleListDeliveries},
		{Route: core.Route{Method: "DELETE", Path: "/_sakumock/deliveries", Description: "Clear recorded firings", Kind: "inspection"}, Handler: s.handleClearDeliveries},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
