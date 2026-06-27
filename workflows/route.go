package workflows

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	return []core.RegisteredRoute{
		// Plans & Subscription
		{Route: core.Route{Method: "GET", Path: "/plans", Description: "List plans", Kind: "api"}, Handler: rl(s.handleListPlans)},
		{Route: core.Route{Method: "GET", Path: "/subscriptions", Description: "Get subscription", Kind: "api"}, Handler: rl(s.handleGetSubscription)},
		{Route: core.Route{Method: "POST", Path: "/subscriptions", Description: "Create subscription", Kind: "api"}, Handler: rl(s.handleCreateSubscription)},
		{Route: core.Route{Method: "DELETE", Path: "/subscriptions", Description: "Delete subscription", Kind: "api"}, Handler: rl(s.handleDeleteSubscription)},

		// Workflows
		{Route: core.Route{Method: "POST", Path: "/workflows", Description: "Create a workflow", Kind: "api"}, Handler: rl(s.handleCreateWorkflow)},
		{Route: core.Route{Method: "GET", Path: "/workflows", Description: "List workflows", Kind: "api"}, Handler: rl(s.handleListWorkflows)},
		{Route: core.Route{Method: "GET", Path: "/workflows/suggest", Description: "List workflow suggestions", Kind: "api"}, Handler: rl(s.handleListWorkflowSuggest)},
		{Route: core.Route{Method: "GET", Path: "/workflows/{id}", Description: "Get a workflow", Kind: "api"}, Handler: rl(s.handleGetWorkflow)},
		{Route: core.Route{Method: "PATCH", Path: "/workflows/{id}", Description: "Update a workflow", Kind: "api"}, Handler: rl(s.handleUpdateWorkflow)},
		{Route: core.Route{Method: "DELETE", Path: "/workflows/{id}", Description: "Delete a workflow", Kind: "api"}, Handler: rl(s.handleDeleteWorkflow)},

		// Revisions
		{Route: core.Route{Method: "POST", Path: "/workflows/{id}/revisions", Description: "Create a revision", Kind: "api"}, Handler: rl(s.handleCreateRevision)},
		{Route: core.Route{Method: "GET", Path: "/workflows/{id}/revisions", Description: "List revisions", Kind: "api"}, Handler: rl(s.handleListRevisions)},
		{Route: core.Route{Method: "GET", Path: "/workflows/{id}/revisions/{revisionId}", Description: "Get a revision", Kind: "api"}, Handler: rl(s.handleGetRevision)},
		{Route: core.Route{Method: "PUT", Path: "/workflows/{id}/revisions/{revisionId}/revision_alias", Description: "Update revision alias", Kind: "api"}, Handler: rl(s.handleUpdateRevisionAlias)},
		{Route: core.Route{Method: "DELETE", Path: "/workflows/{id}/revisions/{revisionId}/revision_alias", Description: "Delete revision alias", Kind: "api"}, Handler: rl(s.handleDeleteRevisionAlias)},

		// Executions
		{Route: core.Route{Method: "POST", Path: "/workflows/{id}/executions", Description: "Create an execution", Kind: "api"}, Handler: rl(s.handleCreateExecution)},
		{Route: core.Route{Method: "GET", Path: "/workflows/{id}/executions", Description: "List executions", Kind: "api"}, Handler: rl(s.handleListExecutions)},
		{Route: core.Route{Method: "GET", Path: "/workflows/{id}/executions/{executionId}", Description: "Get an execution", Kind: "api"}, Handler: rl(s.handleGetExecution)},
		{Route: core.Route{Method: "POST", Path: "/workflows/{id}/executions/{executionId}/cancel", Description: "Cancel an execution", Kind: "api"}, Handler: rl(s.handleCancelExecution)},
		{Route: core.Route{Method: "DELETE", Path: "/workflows/{id}/executions/{executionId}", Description: "Delete an execution", Kind: "api"}, Handler: rl(s.handleDeleteExecution)},
		{Route: core.Route{Method: "GET", Path: "/workflows/{id}/executions/{executionId}/exec_history", Description: "List execution history", Kind: "api"}, Handler: rl(s.handleListExecutionHistory)},
	}
}

func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
