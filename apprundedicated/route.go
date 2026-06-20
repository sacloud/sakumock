package apprundedicated

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	return []core.RegisteredRoute{
		// Clusters
		{Route: core.Route{Method: "POST", Path: "/clusters", Description: "Create cluster", Kind: "api"}, Handler: rl(s.handleCreateCluster)},
		{Route: core.Route{Method: "GET", Path: "/clusters", Description: "List clusters", Kind: "api"}, Handler: rl(s.handleListClusters)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}", Description: "Get cluster", Kind: "api"}, Handler: rl(s.handleGetCluster)},
		{Route: core.Route{Method: "PUT", Path: "/clusters/{clusterID}", Description: "Update cluster", Kind: "api"}, Handler: rl(s.handleUpdateCluster)},
		{Route: core.Route{Method: "DELETE", Path: "/clusters/{clusterID}", Description: "Delete cluster", Kind: "api"}, Handler: rl(s.handleDeleteCluster)},

		// Applications
		{Route: core.Route{Method: "POST", Path: "/applications", Description: "Create application", Kind: "api"}, Handler: rl(s.handleCreateApplication)},
		{Route: core.Route{Method: "GET", Path: "/applications", Description: "List applications", Kind: "api"}, Handler: rl(s.handleListApplications)},
		{Route: core.Route{Method: "GET", Path: "/applications/{applicationID}", Description: "Get application", Kind: "api"}, Handler: rl(s.handleGetApplication)},
		{Route: core.Route{Method: "PUT", Path: "/applications/{applicationID}", Description: "Update application", Kind: "api"}, Handler: rl(s.handleUpdateApplication)},
		{Route: core.Route{Method: "DELETE", Path: "/applications/{applicationID}", Description: "Delete application", Kind: "api"}, Handler: rl(s.handleDeleteApplication)},
		{Route: core.Route{Method: "GET", Path: "/applications/{applicationID}/containers", Description: "Get application containers", Kind: "api"}, Handler: rl(s.handleGetApplicationContainers)},

		// Application Versions
		{Route: core.Route{Method: "POST", Path: "/applications/{applicationID}/versions", Description: "Create version", Kind: "api"}, Handler: rl(s.handleCreateVersion)},
		{Route: core.Route{Method: "GET", Path: "/applications/{applicationID}/versions", Description: "List versions", Kind: "api"}, Handler: rl(s.handleListVersions)},
		{Route: core.Route{Method: "GET", Path: "/applications/{applicationID}/versions/{version}", Description: "Get version", Kind: "api"}, Handler: rl(s.handleGetVersion)},
		{Route: core.Route{Method: "DELETE", Path: "/applications/{applicationID}/versions/{version}", Description: "Delete version", Kind: "api"}, Handler: rl(s.handleDeleteVersion)},

		// Auto Scaling Groups
		{Route: core.Route{Method: "POST", Path: "/clusters/{clusterID}/asg", Description: "Create auto scaling group", Kind: "api"}, Handler: rl(s.handleCreateASG)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg", Description: "List auto scaling groups", Kind: "api"}, Handler: rl(s.handleListASGs)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}", Description: "Get auto scaling group", Kind: "api"}, Handler: rl(s.handleGetASG)},
		{Route: core.Route{Method: "DELETE", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}", Description: "Delete auto scaling group", Kind: "api"}, Handler: rl(s.handleDeleteASG)},

		// Load Balancers
		{Route: core.Route{Method: "POST", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers", Description: "Create load balancer", Kind: "api"}, Handler: rl(s.handleCreateLB)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers", Description: "List load balancers", Kind: "api"}, Handler: rl(s.handleListLBs)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}", Description: "Get load balancer", Kind: "api"}, Handler: rl(s.handleGetLB)},
		{Route: core.Route{Method: "DELETE", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}", Description: "Delete load balancer", Kind: "api"}, Handler: rl(s.handleDeleteLB)},

		// Load Balancer Nodes
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}/load_balancer_nodes", Description: "List load balancer nodes", Kind: "api"}, Handler: rl(s.handleListLBNodes)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/load_balancers/{loadBalancerID}/load_balancer_nodes/{loadBalancerNodeID}", Description: "Get load balancer node", Kind: "api"}, Handler: rl(s.handleGetLBNode)},

		// Worker Nodes
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/worker_nodes", Description: "List worker nodes", Kind: "api"}, Handler: rl(s.handleListWorkerNodes)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/worker_nodes/{workerNodeID}", Description: "Get worker node", Kind: "api"}, Handler: rl(s.handleGetWorkerNode)},
		{Route: core.Route{Method: "PUT", Path: "/clusters/{clusterID}/asg/{autoScalingGroupID}/worker_nodes/{workerNodeID}/draining", Description: "Update worker node draining", Kind: "api"}, Handler: rl(s.handleUpdateWorkerNodeDraining)},

		// Certificates
		{Route: core.Route{Method: "POST", Path: "/clusters/{clusterID}/certificates", Description: "Create certificate", Kind: "api"}, Handler: rl(s.handleCreateCertificate)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/certificates", Description: "List certificates", Kind: "api"}, Handler: rl(s.handleListCertificates)},
		{Route: core.Route{Method: "GET", Path: "/clusters/{clusterID}/certificates/{certificateID}", Description: "Get certificate", Kind: "api"}, Handler: rl(s.handleGetCertificate)},
		{Route: core.Route{Method: "PUT", Path: "/clusters/{clusterID}/certificates/{certificateID}", Description: "Update certificate", Kind: "api"}, Handler: rl(s.handleUpdateCertificate)},
		{Route: core.Route{Method: "DELETE", Path: "/clusters/{clusterID}/certificates/{certificateID}", Description: "Delete certificate", Kind: "api"}, Handler: rl(s.handleDeleteCertificate)},

		// Service Classes
		{Route: core.Route{Method: "GET", Path: "/service_classes/lb", Description: "List LB service classes", Kind: "api"}, Handler: rl(s.handleListLBServiceClasses)},
		{Route: core.Route{Method: "GET", Path: "/service_classes/worker", Description: "List worker service classes", Kind: "api"}, Handler: rl(s.handleListWorkerServiceClasses)},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
