package monitoringsuite

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

// routeTable is the single source of truth driving both buildMux() and
// Routes(). Paths mirror the monitoring-suite OpenAPI spec verbatim (each
// collection path ends in a slash), served at the root because the SDK points
// SAKURA_ENDPOINTS_MONITORING_SUITE at this server's base URL.
func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	route := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{
			Route:   core.Route{Method: method, Path: path, Description: desc, Kind: "api"},
			Handler: rl(h),
		}
	}
	return []core.RegisteredRoute{
		// Alert projects
		route("GET", "/alerts/projects/", "List alert projects", s.handleListAlertProjects),
		route("POST", "/alerts/projects/", "Create an alert project", s.handleCreateAlertProject),
		route("GET", "/alerts/projects/{resource_id}/", "Get an alert project", s.handleReadAlertProject),
		route("PUT", "/alerts/projects/{resource_id}/", "Update an alert project", s.handleUpdateAlertProject),
		route("PATCH", "/alerts/projects/{resource_id}/", "Partially update an alert project", s.handleUpdateAlertProject),
		route("DELETE", "/alerts/projects/{resource_id}/", "Delete an alert project", s.handleDeleteAlertProject),

		// Alert project histories
		route("GET", "/alerts/projects/{project_resource_id}/histories/", "List alert histories", s.handleListProjectHistories),
		route("GET", "/alerts/projects/{project_resource_id}/histories/{uid}/", "Get an alert history", s.handleReadProjectHistory),

		// Alert rules
		route("GET", "/alerts/projects/{project_resource_id}/rules/", "List alert rules", s.handleListAlertRules),
		route("POST", "/alerts/projects/{project_resource_id}/rules/", "Create an alert rule", s.handleCreateAlertRule),
		route("GET", "/alerts/projects/{project_resource_id}/rules/{uid}/", "Get an alert rule", s.handleReadAlertRule),
		route("PUT", "/alerts/projects/{project_resource_id}/rules/{uid}/", "Update an alert rule", s.handleUpdateAlertRule),
		route("PATCH", "/alerts/projects/{project_resource_id}/rules/{uid}/", "Partially update an alert rule", s.handleUpdateAlertRule),
		route("DELETE", "/alerts/projects/{project_resource_id}/rules/{uid}/", "Delete an alert rule", s.handleDeleteAlertRule),
		route("GET", "/alerts/projects/{project_resource_id}/rules/{rule_uid}/histories/", "List alert rule histories", s.handleListRuleHistories),
		route("GET", "/alerts/projects/{project_resource_id}/rules/{rule_uid}/histories/{uid}/", "Get an alert rule history", s.handleReadRuleHistory),

		// Log-measure rules
		route("GET", "/alerts/projects/{project_resource_id}/log-measure-rules/", "List log-measure rules", s.handleListLogMeasureRules),
		route("POST", "/alerts/projects/{project_resource_id}/log-measure-rules/", "Create a log-measure rule", s.handleCreateLogMeasureRule),
		route("GET", "/alerts/projects/{project_resource_id}/log-measure-rules/{uid}/", "Get a log-measure rule", s.handleReadLogMeasureRule),
		route("PUT", "/alerts/projects/{project_resource_id}/log-measure-rules/{uid}/", "Update a log-measure rule", s.handleUpdateLogMeasureRule),
		route("PATCH", "/alerts/projects/{project_resource_id}/log-measure-rules/{uid}/", "Partially update a log-measure rule", s.handleUpdateLogMeasureRule),
		route("DELETE", "/alerts/projects/{project_resource_id}/log-measure-rules/{uid}/", "Delete a log-measure rule", s.handleDeleteLogMeasureRule),

		// Notification targets
		route("GET", "/alerts/projects/{project_resource_id}/notification-targets/", "List notification targets", s.handleListNotificationTargets),
		route("POST", "/alerts/projects/{project_resource_id}/notification-targets/", "Create a notification target", s.handleCreateNotificationTarget),
		route("GET", "/alerts/projects/{project_resource_id}/notification-targets/{uid}/", "Get a notification target", s.handleReadNotificationTarget),
		route("PUT", "/alerts/projects/{project_resource_id}/notification-targets/{uid}/", "Update a notification target", s.handleUpdateNotificationTarget),
		route("PATCH", "/alerts/projects/{project_resource_id}/notification-targets/{uid}/", "Partially update a notification target", s.handleUpdateNotificationTarget),
		route("DELETE", "/alerts/projects/{project_resource_id}/notification-targets/{uid}/", "Delete a notification target", s.handleDeleteNotificationTarget),

		// Notification routings
		route("GET", "/alerts/projects/{project_resource_id}/notification-routings/", "List notification routings", s.handleListNotificationRoutings),
		route("POST", "/alerts/projects/{project_resource_id}/notification-routings/", "Create a notification routing", s.handleCreateNotificationRouting),
		route("PUT", "/alerts/projects/{project_resource_id}/notification-routings/reorder/", "Reorder notification routings", s.handleReorderNotificationRoutings),
		route("GET", "/alerts/projects/{project_resource_id}/notification-routings/{uid}/", "Get a notification routing", s.handleReadNotificationRouting),
		route("PUT", "/alerts/projects/{project_resource_id}/notification-routings/{uid}/", "Update a notification routing", s.handleUpdateNotificationRouting),
		route("PATCH", "/alerts/projects/{project_resource_id}/notification-routings/{uid}/", "Partially update a notification routing", s.handleUpdateNotificationRouting),
		route("DELETE", "/alerts/projects/{project_resource_id}/notification-routings/{uid}/", "Delete a notification routing", s.handleDeleteNotificationRouting),

		// Dashboard projects
		route("GET", "/dashboards/projects/", "List dashboard projects", s.handleListDashboardProjects),
		route("POST", "/dashboards/projects/", "Create a dashboard project", s.handleCreateDashboardProject),
		route("GET", "/dashboards/projects/{resource_id}/", "Get a dashboard project", s.handleReadDashboardProject),
		route("PUT", "/dashboards/projects/{resource_id}/", "Update a dashboard project", s.handleUpdateDashboardProject),
		route("PATCH", "/dashboards/projects/{resource_id}/", "Partially update a dashboard project", s.handleUpdateDashboardProject),
		route("DELETE", "/dashboards/projects/{resource_id}/", "Delete a dashboard project", s.handleDeleteDashboardProject),

		// Log routings
		route("GET", "/logs/routings/", "List log routings", s.handleListLogRoutings),
		route("POST", "/logs/routings/", "Create a log routing", s.handleCreateLogRouting),
		route("GET", "/logs/routings/{uid}/", "Get a log routing", s.handleReadLogRouting),
		route("PUT", "/logs/routings/{uid}/", "Update a log routing", s.handleUpdateLogRouting),
		route("PATCH", "/logs/routings/{uid}/", "Partially update a log routing", s.handleUpdateLogRouting),
		route("DELETE", "/logs/routings/{uid}/", "Delete a log routing", s.handleDeleteLogRouting),

		// Log storages
		route("GET", "/logs/storages/", "List log storages", s.handleListLogStorages),
		route("POST", "/logs/storages/", "Create a log storage", s.handleCreateLogStorage),
		route("GET", "/logs/storages/{resource_id}/", "Get a log storage", s.handleReadLogStorage),
		route("PUT", "/logs/storages/{resource_id}/", "Update a log storage", s.handleUpdateLogStorage),
		route("PATCH", "/logs/storages/{resource_id}/", "Partially update a log storage", s.handleUpdateLogStorage),
		route("DELETE", "/logs/storages/{resource_id}/", "Delete a log storage", s.handleDeleteLogStorage),
		route("POST", "/logs/storages/{resource_id}/set-expire/", "Set log storage expiry", s.handleSetLogStorageExpire),
		route("GET", "/logs/storages/{resource_id}/stats/daily/", "Get log storage daily stats", s.handleLogStorageStatsDaily),
		route("GET", "/logs/storages/{resource_id}/stats/monthly/", "Get log storage monthly stats", s.handleLogStorageStatsMonthly),
		route("GET", "/logs/storages/{log_resource_id}/keys/", "List log storage access keys", s.handleListLogStorageKeys),
		route("POST", "/logs/storages/{log_resource_id}/keys/", "Create a log storage access key", s.handleCreateLogStorageKey),
		route("GET", "/logs/storages/{log_resource_id}/keys/{uid}/", "Get a log storage access key", s.handleReadLogStorageKey),
		route("PUT", "/logs/storages/{log_resource_id}/keys/{uid}/", "Update a log storage access key", s.handleUpdateLogStorageKey),
		route("PATCH", "/logs/storages/{log_resource_id}/keys/{uid}/", "Partially update a log storage access key", s.handleUpdateLogStorageKey),
		route("DELETE", "/logs/storages/{log_resource_id}/keys/{uid}/", "Delete a log storage access key", s.handleDeleteLogStorageKey),

		// Metrics routings
		route("GET", "/metrics/routings/", "List metrics routings", s.handleListMetricsRoutings),
		route("POST", "/metrics/routings/", "Create a metrics routing", s.handleCreateMetricsRouting),
		route("GET", "/metrics/routings/{uid}/", "Get a metrics routing", s.handleReadMetricsRouting),
		route("PUT", "/metrics/routings/{uid}/", "Update a metrics routing", s.handleUpdateMetricsRouting),
		route("PATCH", "/metrics/routings/{uid}/", "Partially update a metrics routing", s.handleUpdateMetricsRouting),
		route("DELETE", "/metrics/routings/{uid}/", "Delete a metrics routing", s.handleDeleteMetricsRouting),

		// Metrics storages
		route("GET", "/metrics/storages/", "List metrics storages", s.handleListMetricsStorages),
		route("POST", "/metrics/storages/", "Create a metrics storage", s.handleCreateMetricsStorage),
		route("GET", "/metrics/storages/{resource_id}/", "Get a metrics storage", s.handleReadMetricsStorage),
		route("PUT", "/metrics/storages/{resource_id}/", "Update a metrics storage", s.handleUpdateMetricsStorage),
		route("PATCH", "/metrics/storages/{resource_id}/", "Partially update a metrics storage", s.handleUpdateMetricsStorage),
		route("DELETE", "/metrics/storages/{resource_id}/", "Delete a metrics storage", s.handleDeleteMetricsStorage),
		route("GET", "/metrics/storages/{resource_id}/stats/daily/", "Get metrics storage daily stats", s.handleMetricsStorageStatsDaily),
		route("GET", "/metrics/storages/{resource_id}/stats/monthly/", "Get metrics storage monthly stats", s.handleMetricsStorageStatsMonthly),
		route("GET", "/metrics/storages/{metrics_resource_id}/keys/", "List metrics storage access keys", s.handleListMetricsStorageKeys),
		route("POST", "/metrics/storages/{metrics_resource_id}/keys/", "Create a metrics storage access key", s.handleCreateMetricsStorageKey),
		route("GET", "/metrics/storages/{metrics_resource_id}/keys/{uid}/", "Get a metrics storage access key", s.handleReadMetricsStorageKey),
		route("PUT", "/metrics/storages/{metrics_resource_id}/keys/{uid}/", "Update a metrics storage access key", s.handleUpdateMetricsStorageKey),
		route("PATCH", "/metrics/storages/{metrics_resource_id}/keys/{uid}/", "Partially update a metrics storage access key", s.handleUpdateMetricsStorageKey),
		route("DELETE", "/metrics/storages/{metrics_resource_id}/keys/{uid}/", "Delete a metrics storage access key", s.handleDeleteMetricsStorageKey),

		// Trace storages
		route("GET", "/traces/storages/", "List trace storages", s.handleListTraceStorages),
		route("POST", "/traces/storages/", "Create a trace storage", s.handleCreateTraceStorage),
		route("GET", "/traces/storages/{resource_id}/", "Get a trace storage", s.handleReadTraceStorage),
		route("PUT", "/traces/storages/{resource_id}/", "Update a trace storage", s.handleUpdateTraceStorage),
		route("PATCH", "/traces/storages/{resource_id}/", "Partially update a trace storage", s.handleUpdateTraceStorage),
		route("DELETE", "/traces/storages/{resource_id}/", "Delete a trace storage", s.handleDeleteTraceStorage),
		route("POST", "/traces/storages/{resource_id}/set-expire/", "Set trace storage expiry", s.handleSetTraceStorageExpire),
		route("GET", "/traces/storages/{resource_id}/stats/daily/", "Get trace storage daily stats", s.handleTraceStorageStatsDaily),
		route("GET", "/traces/storages/{resource_id}/stats/monthly/", "Get trace storage monthly stats", s.handleTraceStorageStatsMonthly),
		route("GET", "/traces/storages/{trace_resource_id}/keys/", "List trace storage access keys", s.handleListTraceStorageKeys),
		route("POST", "/traces/storages/{trace_resource_id}/keys/", "Create a trace storage access key", s.handleCreateTraceStorageKey),
		route("GET", "/traces/storages/{trace_resource_id}/keys/{uid}/", "Get a trace storage access key", s.handleReadTraceStorageKey),
		route("PUT", "/traces/storages/{trace_resource_id}/keys/{uid}/", "Update a trace storage access key", s.handleUpdateTraceStorageKey),
		route("PATCH", "/traces/storages/{trace_resource_id}/keys/{uid}/", "Partially update a trace storage access key", s.handleUpdateTraceStorageKey),
		route("DELETE", "/traces/storages/{trace_resource_id}/keys/{uid}/", "Delete a trace storage access key", s.handleDeleteTraceStorageKey),

		// Publishers (read-only)
		route("GET", "/publishers/", "List publishers", s.handleListPublishers),
		route("GET", "/publishers/{code}/", "Get a publisher", s.handleReadPublisher),

		// Management
		route("GET", "/management/limits/", "Get resource limits", s.handleGetResourceLimits),
		route("POST", "/management/provisioning/initialize/", "Initialize provisioning", s.handleInitializeProvisioning),
		route("GET", "/management/provisioning/state/", "Get provisioning state", s.handleGetProvisioning),
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
