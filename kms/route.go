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
			// Fault injection outermost (an injected fault is an
			// infrastructure-level failure, so it may mask a would-be 429/400),
			// then rate limit, then spec-derived body validation, then the
			// handler. Response validation sits innermost so only what the
			// handler itself produces is checked against the spec.
			Handler: s.fault.Middleware(rl(s.validator.Middleware(method, path, s.respValidator.Middleware(method, path, h)))),
		}
	}
	table := []core.RegisteredRoute{
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
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
