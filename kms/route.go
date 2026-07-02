package kms

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
	return []core.RegisteredRoute{
		route("GET", "/kms/keys", "List all keys", s.handleListKeys),
		route("POST", "/kms/keys", "Create a new key", s.handleCreateKey),
		route("GET", "/kms/keys/{resource_id}", "Get a key", s.handleReadKey),
		route("PUT", "/kms/keys/{resource_id}", "Update a key", s.handleUpdateKey),
		route("DELETE", "/kms/keys/{resource_id}", "Delete a key", s.handleDeleteKey),
		route("POST", "/kms/keys/{resource_id}/rotate", "Rotate a key", s.handleRotateKey),
		route("POST", "/kms/keys/{resource_id}/status", "Change key status", s.handleChangeStatus),
		route("POST", "/kms/keys/{resource_id}/schedule-destruction", "Schedule key destruction", s.handleScheduleDestruction),
		route("POST", "/kms/keys/{resource_id}/encrypt", "Encrypt data with a key", s.handleEncrypt),
		route("POST", "/kms/keys/{resource_id}/decrypt", "Decrypt data with a key", s.handleDecrypt),
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
