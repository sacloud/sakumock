package simplenotification

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "POST", Path: "/commonserviceitem/{id}/simplenotification/message", Description: "Send a notification message to the specified group", Kind: "api"}, Handler: rl(s.handleSendMessage)},
		{Route: core.Route{Method: "GET", Path: "/_sakumock/messages", Description: "List accepted notification messages", Kind: "inspection"}, Handler: s.handleInspectMessages},
		{Route: core.Route{Method: "DELETE", Path: "/_sakumock/messages", Description: "Clear accepted notification messages", Kind: "inspection"}, Handler: s.handleResetMessages},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
