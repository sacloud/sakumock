package simplemq

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.PathValueKey("queueName"), h)
	}
	cp := s.basicAuthMiddleware
	return []core.RegisteredRoute{
		// Data plane
		{Route: core.Route{Method: "POST", Path: "/v1/queues/{queueName}/messages", Description: "Send a message to the queue", Kind: "api"}, Handler: s.authMiddleware(rl(s.handleSend))},
		{Route: core.Route{Method: "GET", Path: "/v1/queues/{queueName}/messages", Description: "Receive messages from the queue", Kind: "api"}, Handler: s.authMiddleware(rl(s.handleReceive))},
		{Route: core.Route{Method: "PUT", Path: "/v1/queues/{queueName}/messages/{messageId}", Description: "Extend the visibility timeout of a message", Kind: "api"}, Handler: s.authMiddleware(rl(s.handleExtendTimeout))},
		{Route: core.Route{Method: "DELETE", Path: "/v1/queues/{queueName}/messages/{messageId}", Description: "Delete a message from the queue", Kind: "api"}, Handler: s.authMiddleware(rl(s.handleDelete))},
		// Control plane
		{Route: core.Route{Method: "POST", Path: "/commonserviceitem", Description: "Create a queue", Kind: "api"}, Handler: cp(s.handleCreateQueue)},
		{Route: core.Route{Method: "GET", Path: "/commonserviceitem", Description: "List queues", Kind: "api"}, Handler: cp(s.handleListQueues)},
		{Route: core.Route{Method: "GET", Path: "/commonserviceitem/{id}", Description: "Get a queue", Kind: "api"}, Handler: cp(s.handleGetQueue)},
		{Route: core.Route{Method: "PUT", Path: "/commonserviceitem/{id}", Description: "Update queue settings", Kind: "api"}, Handler: cp(s.handleConfigQueue)},
		{Route: core.Route{Method: "DELETE", Path: "/commonserviceitem/{id}", Description: "Delete a queue", Kind: "api"}, Handler: cp(s.handleDeleteQueue)},
		{Route: core.Route{Method: "GET", Path: "/commonserviceitem/{id}/simplemq/message-count", Description: "Get message count for a queue", Kind: "api"}, Handler: cp(s.handleGetMessageCount)},
		{Route: core.Route{Method: "PUT", Path: "/commonserviceitem/{id}/simplemq/rotate-apikey", Description: "Rotate the API key for a queue", Kind: "api"}, Handler: cp(s.handleRotateAPIKey)},
		{Route: core.Route{Method: "DELETE", Path: "/commonserviceitem/{id}/simplemq/messages", Description: "Clear all messages from a queue", Kind: "api"}, Handler: cp(s.handleClearMessages)},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
