package simplenotification

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
		route("POST", "/commonserviceitem", "Create a destination, group, or routing", s.handleCreateItem),
		route("GET", "/commonserviceitem", "List destinations, groups, or routings", s.handleListItems),
		route("GET", "/commonserviceitem/{id}", "Get a destination, group, or routing", s.handleGetItem),
		route("PUT", "/commonserviceitem/{id}", "Update a destination, group, or routing", s.handleUpdateItem),
		route("DELETE", "/commonserviceitem/{id}", "Delete a destination, group, or routing", s.handleDeleteItem),
		route("POST", "/commonserviceitem/{id}/simplenotification/message", "Send a notification message to the specified group", s.handleSendMessage),
		{Route: core.Route{Method: "GET", Path: "/_sakumock/messages", Description: "List accepted notification messages", Kind: "inspection"}, Handler: s.handleInspectMessages},
		{Route: core.Route{Method: "DELETE", Path: "/_sakumock/messages", Description: "Clear accepted notification messages", Kind: "inspection"}, Handler: s.handleResetMessages},
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
