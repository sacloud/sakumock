package apprundedicated

import "fmt"

// Spec-expressible constraints (required fields, name patterns, string
// lengths, numeric ranges, enums, array bounds) are enforced by the generated
// bodySchemas table (validate_gen.go) before a handler runs. The functions
// here keep only the domain rules the OpenAPI spec does not express: value
// allow-lists, cross-field comparisons, reserved ranges, and non-empty checks
// for strings the spec gives no minLength.

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
	if len(req.Ports) == 0 {
		return "at least one port is required"
	}
	for _, p := range req.Ports {
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
	if len(req.Image) == 0 {
		return "image must be 1-512 characters"
	}
	return ""
}

func validateCreateASG(req *createASGReq) string {
	// The spec declares no minLength for zone, so the generated validation
	// accepts an empty string; the real API rejects it.
	if req.Zone == "" {
		return "zone is required"
	}
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

func validateCreateCertificate(req *createCertificateReq) string {
	if len(req.CertificatePem) == 0 {
		return "certificatePem must be 1-1000000 characters"
	}
	if len(req.PrivatekeyPem) == 0 {
		return "privatekeyPem must be 1-1000000 characters"
	}
	return ""
}
