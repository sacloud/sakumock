package secretmanager

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	const base = "/secretmanager/vaults/{vault_resource_id}"
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "GET", Path: "/secretmanager/vaults", Description: "List vaults", Kind: "api"}, Handler: rl(s.handleListVaults)},
		{Route: core.Route{Method: "POST", Path: "/secretmanager/vaults", Description: "Create a vault", Kind: "api"}, Handler: rl(s.handleCreateVault)},
		{Route: core.Route{Method: "GET", Path: "/secretmanager/vaults/{vault_resource_id}", Description: "Get a vault", Kind: "api"}, Handler: rl(s.handleGetVault)},
		{Route: core.Route{Method: "PUT", Path: "/secretmanager/vaults/{vault_resource_id}", Description: "Update a vault", Kind: "api"}, Handler: rl(s.handleUpdateVault)},
		{Route: core.Route{Method: "DELETE", Path: "/secretmanager/vaults/{vault_resource_id}", Description: "Delete a vault", Kind: "api"}, Handler: rl(s.handleDeleteVault)},
		{Route: core.Route{Method: "GET", Path: base + "/secrets", Description: "List secrets in a vault", Kind: "api"}, Handler: rl(s.handleListSecrets)},
		{Route: core.Route{Method: "POST", Path: base + "/secrets", Description: "Create or update a secret", Kind: "api"}, Handler: rl(s.handleCreateSecret)},
		{Route: core.Route{Method: "DELETE", Path: base + "/secrets", Description: "Delete a secret", Kind: "api"}, Handler: rl(s.handleDeleteSecret)},
		{Route: core.Route{Method: "POST", Path: base + "/secrets/unveil", Description: "Reveal a secret value", Kind: "api"}, Handler: rl(s.handleUnveil)},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
