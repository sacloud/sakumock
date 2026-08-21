package addon

import (
	"net/http"
	"strings"

	"github.com/sacloud/sakumock/core"
)

// routeTable registers the same resource-group lifecycle for every add-on
// family (see families), which is how the real API is shaped: one path prefix
// per service, identical operations underneath. Paths match the OpenAPI spec
// exactly — the addon SDK client uses SAKURA_ENDPOINTS_ADDON as the whole API
// root URL, so there is no prefix to add.
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

	var table []core.RegisteredRoute
	for _, f := range families {
		a := article(f.label) + " " + f.label
		table = append(table,
			route("GET", f.path, "List "+f.label+" resources", s.handleList(f)),
			route("POST", f.path, "Create "+a+" resource", s.handleCreate(f)),
			route("GET", f.path+"/{resourceGroupName}", "Get "+a+" resource", s.handleRead(f)),
			route("DELETE", f.path+"/{resourceGroupName}", "Delete "+a+" resource", s.handleDelete(f)),
		)
		if f.hasStatus {
			table = append(table, route("GET", f.path+"/status/{resourceGroupName}/{deploymentName}",
				"Get the deployment status of "+a+" resource", s.handleStatus(f)))
		}
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// article picks the indefinite article for a family label, so route
// descriptions read "an AI service resource" but "a CDN service resource".
func article(label string) string {
	if strings.ContainsRune("AEIOU", rune(label[0])) || strings.ContainsRune("aeiou", rune(label[0])) {
		return "an"
	}
	return "a"
}

// Routes describes every HTTP endpoint the server registers.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
