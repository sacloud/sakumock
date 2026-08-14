package secretmanager

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
	const base = "/secretmanager/vaults/{vault_resource_id}"
	table := []core.RegisteredRoute{
		route("GET", "/secretmanager/vaults", "List vaults", s.handleListVaults),
		route("POST", "/secretmanager/vaults", "Create a vault", s.handleCreateVault),
		route("GET", base, "Get a vault", s.handleGetVault),
		route("PUT", base, "Update a vault", s.handleUpdateVault),
		route("DELETE", base, "Delete a vault", s.handleDeleteVault),
		route("GET", base+"/secrets", "List secrets in a vault", s.handleListSecrets),
		route("POST", base+"/secrets", "Create or update a secret", s.handleCreateSecret),
		route("DELETE", base+"/secrets", "Delete a secret", s.handleDeleteSecret),
		route("POST", base+"/secrets/unveil", "Reveal a secret value", s.handleUnveil),
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes describes every HTTP endpoint the server registers.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
