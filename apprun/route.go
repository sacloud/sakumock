package apprun

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "GET", Path: "/user", Description: "Get user info", Kind: "api"}, Handler: rl(s.handleGetUser)},
		{Route: core.Route{Method: "POST", Path: "/user", Description: "Create user", Kind: "api"}, Handler: rl(s.handlePostUser)},
		{Route: core.Route{Method: "GET", Path: "/applications", Description: "List applications", Kind: "api"}, Handler: rl(s.handleListApplications)},
		{Route: core.Route{Method: "POST", Path: "/applications", Description: "Create application", Kind: "api"}, Handler: rl(s.handlePostApplication)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}", Description: "Get application", Kind: "api"}, Handler: rl(s.handleGetApplication)},
		{Route: core.Route{Method: "PATCH", Path: "/applications/{id}", Description: "Update application", Kind: "api"}, Handler: rl(s.handlePatchApplication)},
		{Route: core.Route{Method: "DELETE", Path: "/applications/{id}", Description: "Delete application", Kind: "api"}, Handler: rl(s.handleDeleteApplication)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}/status", Description: "Get application status", Kind: "api"}, Handler: rl(s.handleGetApplicationStatus)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}/versions", Description: "List versions", Kind: "api"}, Handler: rl(s.handleListVersions)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}/versions/{version_id}", Description: "Get version", Kind: "api"}, Handler: rl(s.handleGetVersion)},
		{Route: core.Route{Method: "DELETE", Path: "/applications/{id}/versions/{version_id}", Description: "Delete version", Kind: "api"}, Handler: rl(s.handleDeleteVersion)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}/versions/{version_id}/status", Description: "Get version status", Kind: "api"}, Handler: rl(s.handleGetVersionStatus)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}/traffics", Description: "Get traffic distribution", Kind: "api"}, Handler: rl(s.handleListTraffics)},
		{Route: core.Route{Method: "PUT", Path: "/applications/{id}/traffics", Description: "Update traffic distribution", Kind: "api"}, Handler: rl(s.handlePutTraffics)},
		{Route: core.Route{Method: "GET", Path: "/applications/{id}/packet_filter", Description: "Get packet filter", Kind: "api"}, Handler: rl(s.handleGetPacketFilter)},
		{Route: core.Route{Method: "PATCH", Path: "/applications/{id}/packet_filter", Description: "Update packet filter", Kind: "api"}, Handler: rl(s.handlePatchPacketFilter)},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
