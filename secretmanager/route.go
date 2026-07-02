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
			// Rate limit outermost, then spec-derived body validation, then
			// the handler.
			Handler: rl(s.validator.Middleware(method, path, h)),
		}
	}
	const base = "/secretmanager/vaults/{vault_resource_id}"
	return []core.RegisteredRoute{
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
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
