package workflows

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
			// handler.
			Handler: s.fault.Middleware(rl(s.validator.Middleware(method, path, h))),
		}
	}
	return []core.RegisteredRoute{
		// Plans & Subscription
		route("GET", "/plans", "List plans", s.handleListPlans),
		route("GET", "/subscriptions", "Get subscription", s.handleGetSubscription),
		route("POST", "/subscriptions", "Create subscription", s.handleCreateSubscription),
		route("DELETE", "/subscriptions", "Delete subscription", s.handleDeleteSubscription),

		// Workflows
		route("POST", "/workflows", "Create a workflow", s.handleCreateWorkflow),
		route("GET", "/workflows", "List workflows", s.handleListWorkflows),
		route("GET", "/workflows/suggest", "List workflow suggestions", s.handleListWorkflowSuggest),
		route("GET", "/workflows/{id}", "Get a workflow", s.handleGetWorkflow),
		route("PATCH", "/workflows/{id}", "Update a workflow", s.handleUpdateWorkflow),
		route("DELETE", "/workflows/{id}", "Delete a workflow", s.handleDeleteWorkflow),

		// Revisions
		route("POST", "/workflows/{id}/revisions", "Create a revision", s.handleCreateRevision),
		route("GET", "/workflows/{id}/revisions", "List revisions", s.handleListRevisions),
		route("GET", "/workflows/{id}/revisions/{revisionId}", "Get a revision", s.handleGetRevision),
		route("PUT", "/workflows/{id}/revisions/{revisionId}/revision_alias", "Update revision alias", s.handleUpdateRevisionAlias),
		route("DELETE", "/workflows/{id}/revisions/{revisionId}/revision_alias", "Delete revision alias", s.handleDeleteRevisionAlias),

		// Executions
		route("POST", "/workflows/{id}/executions", "Create an execution", s.handleCreateExecution),
		route("GET", "/workflows/{id}/executions", "List executions", s.handleListExecutions),
		route("GET", "/workflows/{id}/executions/{executionId}", "Get an execution", s.handleGetExecution),
		route("POST", "/workflows/{id}/executions/{executionId}/cancel", "Cancel an execution", s.handleCancelExecution),
		route("DELETE", "/workflows/{id}/executions/{executionId}", "Delete an execution", s.handleDeleteExecution),
		route("GET", "/workflows/{id}/executions/{executionId}/exec_history", "List execution history", s.handleListExecutionHistory),
	}
}

func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
