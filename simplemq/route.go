package simplemq

import (
	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "POST", Path: "/v1/queues/{queueName}/messages", Description: "Send a message to the queue", Kind: "api"}, Handler: s.authMiddleware(s.handleSend)},
		{Route: core.Route{Method: "GET", Path: "/v1/queues/{queueName}/messages", Description: "Receive messages from the queue", Kind: "api"}, Handler: s.authMiddleware(s.handleReceive)},
		{Route: core.Route{Method: "PUT", Path: "/v1/queues/{queueName}/messages/{messageId}", Description: "Extend the visibility timeout of a message", Kind: "api"}, Handler: s.authMiddleware(s.handleExtendTimeout)},
		{Route: core.Route{Method: "DELETE", Path: "/v1/queues/{queueName}/messages/{messageId}", Description: "Delete a message from the queue", Kind: "api"}, Handler: s.authMiddleware(s.handleDelete)},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
