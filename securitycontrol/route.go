package securitycontrol

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
		route("POST", "/activation", "Activate security control for the project", s.handleCreateActivation),
		route("GET", "/activation", "Get the project's activation state", s.handleReadActivation),
		route("PUT", "/activation", "Update the project's activation state", s.handleUpdateActivation),
		route("GET", "/automated_actions", "List automated actions", s.handleListAutomatedActions),
		route("POST", "/automated_actions", "Create an automated action", s.handleCreateAutomatedAction),
		route("GET", "/automated_actions/{actionId}", "Get an automated action", s.handleReadAutomatedAction),
		route("PUT", "/automated_actions/{actionId}", "Update an automated action", s.handleUpdateAutomatedAction),
		route("DELETE", "/automated_actions/{actionId}", "Delete an automated action", s.handleDeleteAutomatedAction),
		route("GET", "/evaluation_rules", "List evaluation rules", s.handleListEvaluationRules),
		route("GET", "/evaluation_rules/{evaluationRuleId}", "Get an evaluation rule", s.handleReadEvaluationRule),
		route("PUT", "/evaluation_rules/{evaluationRuleId}", "Update an evaluation rule", s.handleUpdateEvaluationRule),
	}
	table = append(table, core.UnimplementedRoutes(unimplementedRoutes, writeError)...)
	return append(table, core.InspectionRoutes(s.respValidator)...)
}

// Routes describes every HTTP endpoint the server registers.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
