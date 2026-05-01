package secretmanager

import (
	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	const base = "/secretmanager/vaults/{vault_resource_id}"
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "GET", Path: base + "/secrets", Description: "List secrets in a vault", Kind: "api"}, Handler: s.handleListSecrets},
		{Route: core.Route{Method: "POST", Path: base + "/secrets", Description: "Create or update a secret", Kind: "api"}, Handler: s.handleCreateSecret},
		{Route: core.Route{Method: "DELETE", Path: base + "/secrets", Description: "Delete a secret", Kind: "api"}, Handler: s.handleDeleteSecret},
		{Route: core.Route{Method: "POST", Path: base + "/secrets/unveil", Description: "Reveal a secret value", Kind: "api"}, Handler: s.handleUnveil},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
