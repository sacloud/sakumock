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
			// Fault injection outermost (an injected fault is an
			// infrastructure-level failure, so it may mask a would-be 429/400),
			// then rate limit, then spec-derived body validation, then the
			// handler. Response validation sits innermost so only what the
			// handler itself produces is checked against the spec.
			Handler: s.fault.Middleware(rl(s.validator.Middleware(method, path, s.respValidator.Middleware(method, path, h)))),
		}
	}
	table := []core.RegisteredRoute{
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
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes describes every HTTP endpoint the server registers.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
