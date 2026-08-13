package apprun

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
		route("GET", "/user", "Get user info", s.handleGetUser),
		route("POST", "/user", "Create user", s.handlePostUser),
		route("GET", "/applications", "List applications", s.handleListApplications),
		route("POST", "/applications", "Create application", s.handlePostApplication),
		route("GET", "/applications/{id}", "Get application", s.handleGetApplication),
		route("PATCH", "/applications/{id}", "Update application", s.handlePatchApplication),
		route("DELETE", "/applications/{id}", "Delete application", s.handleDeleteApplication),
		route("GET", "/applications/{id}/status", "Get application status", s.handleGetApplicationStatus),
		route("GET", "/applications/{id}/versions", "List versions", s.handleListVersions),
		route("GET", "/applications/{id}/versions/{version_id}", "Get version", s.handleGetVersion),
		route("DELETE", "/applications/{id}/versions/{version_id}", "Delete version", s.handleDeleteVersion),
		route("GET", "/applications/{id}/versions/{version_id}/status", "Get version status", s.handleGetVersionStatus),
		route("GET", "/applications/{id}/traffics", "Get traffic distribution", s.handleListTraffics),
		route("PUT", "/applications/{id}/traffics", "Update traffic distribution", s.handlePutTraffics),
		route("GET", "/applications/{id}/packet_filter", "Get packet filter", s.handleGetPacketFilter),
		route("PATCH", "/applications/{id}/packet_filter", "Update packet filter", s.handlePatchPacketFilter),
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
