package kms

import (
	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	return []core.RegisteredRoute{
		{Route: core.Route{Method: "GET", Path: "/kms/keys", Description: "List all keys", Kind: "api"}, Handler: s.handleListKeys},
		{Route: core.Route{Method: "POST", Path: "/kms/keys", Description: "Create a new key", Kind: "api"}, Handler: s.handleCreateKey},
		{Route: core.Route{Method: "GET", Path: "/kms/keys/{resource_id}", Description: "Get a key", Kind: "api"}, Handler: s.handleReadKey},
		{Route: core.Route{Method: "PUT", Path: "/kms/keys/{resource_id}", Description: "Update a key", Kind: "api"}, Handler: s.handleUpdateKey},
		{Route: core.Route{Method: "DELETE", Path: "/kms/keys/{resource_id}", Description: "Delete a key", Kind: "api"}, Handler: s.handleDeleteKey},
		{Route: core.Route{Method: "POST", Path: "/kms/keys/{resource_id}/rotate", Description: "Rotate a key", Kind: "api"}, Handler: s.handleRotateKey},
		{Route: core.Route{Method: "POST", Path: "/kms/keys/{resource_id}/status", Description: "Change key status", Kind: "api"}, Handler: s.handleChangeStatus},
		{Route: core.Route{Method: "POST", Path: "/kms/keys/{resource_id}/schedule-destruction", Description: "Schedule key destruction", Kind: "api"}, Handler: s.handleScheduleDestruction},
		{Route: core.Route{Method: "POST", Path: "/kms/keys/{resource_id}/encrypt", Description: "Encrypt data with a key", Kind: "api"}, Handler: s.handleEncrypt},
		{Route: core.Route{Method: "POST", Path: "/kms/keys/{resource_id}/decrypt", Description: "Decrypt data with a key", Kind: "api"}, Handler: s.handleDecrypt},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
