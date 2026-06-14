package eventbus

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "POST", Path: "/commonserviceitem", Description: "Create a process configuration, schedule, or trigger", Kind: "api"}, Handler: rl(s.handleCreateItem)},
		{Route: core.Route{Method: "GET", Path: "/commonserviceitem", Description: "List process configurations, schedules, or triggers", Kind: "api"}, Handler: rl(s.handleListItems)},
		{Route: core.Route{Method: "GET", Path: "/commonserviceitem/{id}", Description: "Get a process configuration, schedule, or trigger", Kind: "api"}, Handler: rl(s.handleGetItem)},
		{Route: core.Route{Method: "PUT", Path: "/commonserviceitem/{id}", Description: "Update a process configuration, schedule, or trigger", Kind: "api"}, Handler: rl(s.handleUpdateItem)},
		{Route: core.Route{Method: "DELETE", Path: "/commonserviceitem/{id}", Description: "Delete a process configuration, schedule, or trigger", Kind: "api"}, Handler: rl(s.handleDeleteItem)},
		{Route: core.Route{Method: "PUT", Path: "/commonserviceitem/{id}/eventbus/processconfiguration/set-secret", Description: "Set the secret of a process configuration", Kind: "api"}, Handler: rl(s.handleSetSecret)},

		// Mock-only data-plane endpoints (not rate-limited, like other inspection helpers).
		{Route: core.Route{Method: "POST", Path: "/_sakumock/events", Description: "Inject an event and fire matching triggers", Kind: "inspection"}, Handler: s.handleInjectEvent},
		{Route: core.Route{Method: "POST", Path: "/_sakumock/tick", Description: "Evaluate schedules and fire those due (optional ?at=<epoch-ms>)", Kind: "inspection"}, Handler: s.handleTick},
		{Route: core.Route{Method: "GET", Path: "/_sakumock/deliveries", Description: "List recorded firings", Kind: "inspection"}, Handler: s.handleListDeliveries},
		{Route: core.Route{Method: "DELETE", Path: "/_sakumock/deliveries", Description: "Clear recorded firings", Kind: "inspection"}, Handler: s.handleClearDeliveries},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
