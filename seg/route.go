package seg

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
		route("GET", "/appliance", "List all Service Endpoint Gateways", s.handleList),
		route("POST", "/appliance", "Create a new Service Endpoint Gateway", s.handleCreate),
		route("GET", "/appliance/{applianceID}", "Get a Service Endpoint Gateway", s.handleRead),
		route("PUT", "/appliance/{applianceID}", "Update a Service Endpoint Gateway", s.handleUpdate),
		route("DELETE", "/appliance/{applianceID}", "Delete a Service Endpoint Gateway", s.handleDelete),
		route("PUT", "/appliance/{applianceID}/config", "Apply a Service Endpoint Gateway's configuration", s.handleApply),
		route("GET", "/appliance/{applianceID}/interface/{interfaceID}", "Get a Service Endpoint Gateway interface", s.handleReadInterface),
		route("GET", "/appliance/{applianceID}/power", "Get a Service Endpoint Gateway's power status", s.handleReadPowerStatus),
		route("PUT", "/appliance/{applianceID}/power", "Power on a Service Endpoint Gateway", s.handlePowerOn),
		route("DELETE", "/appliance/{applianceID}/power", "Power off a Service Endpoint Gateway", s.handlePowerOff),
		route("PUT", "/appliance/{applianceID}/reset", "Reset a Service Endpoint Gateway's power status", s.handleReset),
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes describes every HTTP endpoint the server registers.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
