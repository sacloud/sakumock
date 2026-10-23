package apprundedicated

import "fmt"

// Spec-expressible constraints (required fields, name patterns, string
// lengths, numeric ranges, enums, array bounds) are enforced by the generated
// bodySchemas table (validate_gen.go) before a handler runs, and non-empty
// checks for strings the spec gives no minLength are overlaid via
// validate_overrides.go. The functions here keep only the domain rules the
// OpenAPI spec does not express: value allow-lists, cross-field comparisons,
// and reserved ranges.

var validWorkerServiceClassPaths = map[string]bool{
	"cloud/apprun/dedicated/worker/1vcpu_2gb": true,
	"cloud/apprun/dedicated/worker/2vcpu_2gb": true,
	"cloud/apprun/dedicated/worker/4vcpu_4gb": true,
	"cloud/apprun/dedicated/worker/8vcpu_8gb": true,
}

var validLBServiceClassPaths = map[string]bool{
	"cloud/apprun/dedicated/lb/1vcpu_2gb":    true,
	"cloud/apprun/dedicated/lb/2vcpu_2gb":    true,
	"cloud/apprun/dedicated/lb-ha/1vcpu_2gb": true,
	"cloud/apprun/dedicated/lb-ha/2vcpu_2gb": true,
}

func validateCreateCluster(req *createClusterReq) string {
	return validateClusterPorts(req.Ports)
}

// validateUpdateCluster checks ports only when the request carries them;
// omitting ports keeps the cluster's current ones.
func validateUpdateCluster(req *updateClusterReq) string {
	if req.Ports == nil {
		return ""
	}
	return validateClusterPorts(req.Ports)
}

func validateClusterPorts(ports []clusterPortJSON) string {
	if len(ports) == 0 {
		return "at least one port is required"
	}
	for _, p := range ports {
		if p.Port >= 5950 && p.Port <= 5959 {
			return fmt.Sprintf("port %d is reserved (5950-5959)", p.Port)
		}
	}
	return ""
}

func validateCreateVersion(req *createVersionReq) string {
	if req.MinScale != nil && req.MaxScale != nil && *req.MinScale > *req.MaxScale {
		return "minScale must be less than or equal to maxScale"
	}
	return ""
}

func validateCreateASG(req *createASGReq) string {
	if !validWorkerServiceClassPaths[req.WorkerServiceClassPath] {
		return fmt.Sprintf("invalid workerServiceClassPath: %q", req.WorkerServiceClassPath)
	}
	if req.MinNodes > req.MaxNodes {
		return "minNodes must be less than or equal to maxNodes"
	}
	return ""
}

func validateCreateLB(req *createLBReq) string {
	if !validLBServiceClassPaths[req.ServiceClassPath] {
		return fmt.Sprintf("invalid serviceClassPath: %q", req.ServiceClassPath)
	}
	return ""
}
