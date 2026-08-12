package apprundedicated

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
		// Clusters
		route("POST", "/clusters", "Create cluster", s.handleCreateCluster),
		route("GET", "/clusters", "List clusters", s.handleListClusters),
		route("GET", "/clusters/{clusterID}", "Get cluster", s.handleGetCluster),
		route("PUT", "/clusters/{clusterID}", "Update cluster", s.handleUpdateCluster),
		route("DELETE", "/clusters/{clusterID}", "Delete cluster", s.handleDeleteCluster),

		// Applications
		route("POST", "/applications", "Create application", s.handleCreateApplication),
		route("GET", "/applications", "List applications", s.handleListApplications),
		route("GET", "/applications/{applicationID}", "Get application", s.handleGetApplication),
		route("PUT", "/applications/{applicationID}", "Update application", s.handleUpdateApplication),
		route("DELETE", "/applications/{applicationID}", "Delete application", s.handleDeleteApplication),
		route("GET", "/applications/{applicationID}/containers", "Get application containers", s.handleGetApplicationContainers),

		// Application Versions
		route("POST", "/applications/{applicationID}/versions", "Create version", s.handleCreateVersion),
		route("GET", "/applications/{applicationID}/versions", "List versions", s.handleListVersions),
		route("GET", "/applications/{applicationID}/versions/{version}", "Get version", s.handleGetVersion),
		route("DELETE", "/applications/{applicationID}/versions/{version}", "Delete version", s.handleDeleteVersion),

		// Auto Scaling Groups
		route("POST", "/clusters/{clusterID}/asg", "Create auto scaling group", s.handleCreateASG),
		route("GET", "/clusters/{clusterID}/asg", "List auto scaling groups", s.handleListASGs),
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}", "Get auto scaling group", s.handleGetASG),
		route("DELETE", "/clusters/{clusterID}/asg/{autoScalingGroupID}", "Delete auto scaling group", s.handleDeleteASG),

		// Load Balancers
		route("POST", "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers", "Create load balancer", s.handleCreateLB),
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers", "List load balancers", s.handleListLBs),
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}", "Get load balancer", s.handleGetLB),
		route("DELETE", "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}", "Delete load balancer", s.handleDeleteLB),

		// Load Balancer Nodes
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}/load_balancer_nodes", "List load balancer nodes", s.handleListLBNodes),
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}/load_balancer_nodes/{loadBalancerNodeID}", "Get load balancer node", s.handleGetLBNode),

		// Worker Nodes
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}/worker_nodes", "List worker nodes", s.handleListWorkerNodes),
		route("GET", "/clusters/{clusterID}/asg/{autoScalingGroupID}/worker_nodes/{workerNodeID}", "Get worker node", s.handleGetWorkerNode),
		route("PUT", "/clusters/{clusterID}/asg/{autoScalingGroupID}/worker_nodes/{workerNodeID}/draining", "Update worker node draining", s.handleUpdateWorkerNodeDraining),

		// Certificates
		route("POST", "/clusters/{clusterID}/certificates", "Create certificate", s.handleCreateCertificate),
		route("GET", "/clusters/{clusterID}/certificates", "List certificates", s.handleListCertificates),
		route("GET", "/clusters/{clusterID}/certificates/{certificateID}", "Get certificate", s.handleGetCertificate),
		route("PUT", "/clusters/{clusterID}/certificates/{certificateID}", "Update certificate", s.handleUpdateCertificate),
		route("DELETE", "/clusters/{clusterID}/certificates/{certificateID}", "Delete certificate", s.handleDeleteCertificate),

		// Service Classes
		route("GET", "/service_classes/lb", "List LB service classes", s.handleListLBServiceClasses),
		route("GET", "/service_classes/worker", "List worker service classes", s.handleListWorkerServiceClasses),
	}
	return append(table, core.SpecViolationRoutes(s.respValidator)...)
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
