package simplemq

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.PathValueKey("queueName"), h)
	}
	// Data-plane routes: bearer auth outermost, then rate limit, then
	// spec-derived body validation, then the handler.
	dp := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{
			Route:   core.Route{Method: method, Path: path, Description: desc, Kind: "api"},
			Handler: s.authMiddleware(rl(s.validator.Middleware(method, path, h))),
		}
	}
	// Control-plane routes: basic auth outermost, then spec-derived body
	// validation (standard-error shape), then the handler.
	cp := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{
			Route:   core.Route{Method: method, Path: path, Description: desc, Kind: "api"},
			Handler: s.basicAuthMiddleware(s.cpValidator.Middleware(method, path, h)),
		}
	}
	return []core.RegisteredRoute{
		// Data plane
		dp("POST", "/v1/queues/{queueName}/messages", "Send a message to the queue", s.handleSend),
		dp("GET", "/v1/queues/{queueName}/messages", "Receive messages from the queue", s.handleReceive),
		dp("PUT", "/v1/queues/{queueName}/messages/{messageId}", "Extend the visibility timeout of a message", s.handleExtendTimeout),
		dp("DELETE", "/v1/queues/{queueName}/messages/{messageId}", "Delete a message from the queue", s.handleDelete),
		// Control plane
		cp("POST", "/commonserviceitem", "Create a queue", s.handleCreateQueue),
		cp("GET", "/commonserviceitem", "List queues", s.handleListQueues),
		cp("GET", "/commonserviceitem/{id}", "Get a queue", s.handleGetQueue),
		cp("PUT", "/commonserviceitem/{id}", "Update queue settings", s.handleConfigQueue),
		cp("DELETE", "/commonserviceitem/{id}", "Delete a queue", s.handleDeleteQueue),
		cp("GET", "/commonserviceitem/{id}/simplemq/message-count", "Get message count for a queue", s.handleGetMessageCount),
		cp("PUT", "/commonserviceitem/{id}/simplemq/rotate-apikey", "Rotate the API key for a queue", s.handleRotateAPIKey),
		cp("DELETE", "/commonserviceitem/{id}/simplemq/messages", "Clear all messages from a queue", s.handleClearMessages),
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
