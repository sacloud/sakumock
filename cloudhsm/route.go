package cloudhsm

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

// zonePrefix is prepended to every API route. Unlike KMS/SecretManager
// (global services, where SAKURA_ENDPOINTS_* overrides the entire API root
// URL), the cloudhsm-api-go client always joins "<endpoint>/<zone>/api/cloud/1.1/"
// regardless of whether the endpoint came from an override — CloudHSM
// hardware is physically zone-scoped in the real product. {zone} is a
// wildcard: the mock does not care which zone the client sends.
const zonePrefix = "/{zone}/api/cloud/1.1"

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	route := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		path = zonePrefix + path
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
		route("GET", "/cloudhsm/cloudhsms", "List all CloudHSMs", s.handleListCloudHSMs),
		route("POST", "/cloudhsm/cloudhsms", "Create a new CloudHSM", s.handleCreateCloudHSM),
		route("GET", "/cloudhsm/cloudhsms/{resource_id}", "Get a CloudHSM", s.handleReadCloudHSM),
		route("PUT", "/cloudhsm/cloudhsms/{resource_id}", "Update a CloudHSM", s.handleUpdateCloudHSM),
		route("DELETE", "/cloudhsm/cloudhsms/{resource_id}", "Delete a CloudHSM", s.handleDeleteCloudHSM),

		route("GET", "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients", "List clients of a CloudHSM", s.handleListClients),
		route("POST", "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients", "Create a client of a CloudHSM", s.handleCreateClient),
		route("GET", "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}", "Get a client", s.handleReadClient),
		route("PUT", "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}", "Update a client", s.handleUpdateClient),
		route("DELETE", "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}", "Delete a client", s.handleDeleteClient),

		route("GET", "/cloudhsm/cloudhsms/{resource_id}/peers", "List peers of a CloudHSM", s.handleListPeers),
		route("POST", "/cloudhsm/cloudhsms/{resource_id}/peers", "Create a peer of a CloudHSM", s.handleCreatePeer),
		route("DELETE", "/cloudhsm/cloudhsms/{resource_id}/peers/{peer_id}", "Delete a peer", s.handleDeletePeer),

		route("GET", "/cloudhsm/licenses", "List all CloudHSM software licenses", s.handleListLicenses),
		route("POST", "/cloudhsm/licenses", "Create a new CloudHSM software license", s.handleCreateLicense),
		route("GET", "/cloudhsm/licenses/{resource_id}", "Get a CloudHSM software license", s.handleReadLicense),
		route("PUT", "/cloudhsm/licenses/{resource_id}", "Update a CloudHSM software license", s.handleUpdateLicense),
		route("DELETE", "/cloudhsm/licenses/{resource_id}", "Delete a CloudHSM software license", s.handleDeleteLicense),
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes describes every HTTP endpoint the server registers.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
