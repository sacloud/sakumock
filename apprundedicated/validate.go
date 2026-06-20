package apprundedicated

import (
	"fmt"
	"regexp"
)

var (
	namePattern     = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	certNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	emailPattern    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

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
	if msg := validateName(req.Name); msg != "" {
		return msg
	}
	if len(req.ServicePrincipalID) != 12 {
		return "servicePrincipalID must be exactly 12 characters"
	}
	if len(req.Ports) == 0 {
		return "at least one port is required"
	}
	if len(req.Ports) > 5 {
		return "maximum 5 ports allowed"
	}
	for _, p := range req.Ports {
		if msg := validateClusterPort(p); msg != "" {
			return msg
		}
	}
	if req.LetsEncryptEmail != nil && !emailPattern.MatchString(*req.LetsEncryptEmail) {
		return "letsEncryptEmail must be a valid email address"
	}
	return ""
}

func validateUpdateCluster(req *updateClusterReq) string {
	if req.LetsEncryptEmail != nil && !emailPattern.MatchString(*req.LetsEncryptEmail) {
		return "letsEncryptEmail must be a valid email address"
	}
	return ""
}

func validateClusterPort(p clusterPortJSON) string {
	if p.Port < 1 {
		return "port must be between 1 and 65535"
	}
	if p.Port >= 5950 && p.Port <= 5959 {
		return fmt.Sprintf("port %d is reserved (5950-5959)", p.Port)
	}
	switch p.Protocol {
	case "http", "https", "tcp":
	default:
		return fmt.Sprintf("protocol must be one of: http, https, tcp (got %q)", p.Protocol)
	}
	return ""
}

func validateCreateApplication(req *createApplicationReq) string {
	if msg := validateName(req.Name); msg != "" {
		return msg
	}
	if req.ClusterID == "" {
		return "clusterID is required"
	}
	return ""
}

func validateCreateVersion(req *createVersionReq) string {
	if req.CPU < 100 || req.CPU > 64000 {
		return "cpu must be between 100 and 64000"
	}
	if req.Memory < 128 || req.Memory > 131072 {
		return "memory must be between 128 and 131072"
	}
	switch req.ScalingMode {
	case "manual", "cpu":
	default:
		return fmt.Sprintf("scalingMode must be one of: manual, cpu (got %q)", req.ScalingMode)
	}
	if req.FixedScale != nil && (*req.FixedScale < 1 || *req.FixedScale > 50) {
		return "fixedScale must be between 1 and 50"
	}
	if req.MinScale != nil && (*req.MinScale < 1 || *req.MinScale > 50) {
		return "minScale must be between 1 and 50"
	}
	if req.MaxScale != nil && (*req.MaxScale < 1 || *req.MaxScale > 50) {
		return "maxScale must be between 1 and 50"
	}
	if req.MinScale != nil && req.MaxScale != nil && *req.MinScale > *req.MaxScale {
		return "minScale must be less than or equal to maxScale"
	}
	if req.ScaleInThreshold != nil && (*req.ScaleInThreshold < 30 || *req.ScaleInThreshold > 70) {
		return "scaleInThreshold must be between 30 and 70"
	}
	if req.ScaleOutThreshold != nil && (*req.ScaleOutThreshold < 50 || *req.ScaleOutThreshold > 99) {
		return "scaleOutThreshold must be between 50 and 99"
	}
	if len(req.Image) == 0 || len(req.Image) > 512 {
		return "image must be 1-512 characters"
	}
	if len(req.Cmd) > 20 {
		return "maximum 20 cmd entries allowed"
	}
	if req.RegistryUsername != nil && len(*req.RegistryUsername) > 255 {
		return "registryUsername must be at most 255 characters"
	}
	if req.RegistryPassword != nil && len(*req.RegistryPassword) > 255 {
		return "registryPassword must be at most 255 characters"
	}
	if len(req.ExposedPorts) > 5 {
		return "maximum 5 exposed ports allowed"
	}
	for _, p := range req.ExposedPorts {
		if msg := validateExposedPort(p); msg != "" {
			return msg
		}
	}
	if len(req.Env) > 50 {
		return "maximum 50 environment variables allowed"
	}
	for _, e := range req.Env {
		if msg := validateEnvVar(e); msg != "" {
			return msg
		}
	}
	return ""
}

func validateExposedPort(p exposedPortJSON) string {
	if p.TargetPort < 1 {
		return "targetPort must be between 1 and 65535"
	}
	if p.LoadBalancerPort != nil && *p.LoadBalancerPort < 1 {
		return "loadBalancerPort must be between 1 and 65535"
	}
	if len(p.Host) > 5 {
		return "maximum 5 hosts per exposed port"
	}
	if p.HealthCheck != nil {
		if len(p.HealthCheck.Path) > 200 {
			return "healthCheck.path must be at most 200 characters"
		}
		if p.HealthCheck.IntervalSeconds < 3 || p.HealthCheck.IntervalSeconds > 60 {
			return "healthCheck.intervalSeconds must be between 3 and 60"
		}
		if p.HealthCheck.TimeoutSeconds > 60 {
			return "healthCheck.timeoutSeconds must be at most 60"
		}
	}
	return ""
}

func validateEnvVar(e createEnvJSON) string {
	if len(e.Key) < 1 || len(e.Key) > 255 {
		return "environment variable key must be 1-255 characters"
	}
	if e.Value != nil && len(*e.Value) > 4096 {
		return "environment variable value must be at most 4096 characters"
	}
	return ""
}

func validateCreateASG(req *createASGReq) string {
	if msg := validateName(req.Name); msg != "" {
		return msg
	}
	if req.Zone == "" {
		return "zone is required"
	}
	if len(req.WorkerServiceClassPath) < 1 || len(req.WorkerServiceClassPath) > 255 {
		return "workerServiceClassPath must be 1-255 characters"
	}
	if !validWorkerServiceClassPaths[req.WorkerServiceClassPath] {
		return fmt.Sprintf("invalid workerServiceClassPath: %q", req.WorkerServiceClassPath)
	}
	if req.MinNodes < 1 || req.MinNodes > 10 {
		return "minNodes must be between 1 and 10"
	}
	if req.MaxNodes < 1 || req.MaxNodes > 10 {
		return "maxNodes must be between 1 and 10"
	}
	if req.MinNodes > req.MaxNodes {
		return "minNodes must be less than or equal to maxNodes"
	}
	if len(req.NameServers) > 0 && (len(req.NameServers) < 1 || len(req.NameServers) > 3) {
		return "nameServers must have 1-3 entries"
	}
	if len(req.Interfaces) < 1 || len(req.Interfaces) > 5 {
		return "interfaces must have 1-5 entries"
	}
	for _, iface := range req.Interfaces {
		if len(iface.IpPool) > 20 {
			return "maximum 20 IP ranges per interface"
		}
		if iface.NetmaskLen != nil && (*iface.NetmaskLen < 8 || *iface.NetmaskLen > 29) {
			return "netmaskLen must be between 8 and 29"
		}
	}
	return ""
}

func validateCreateLB(req *createLBReq) string {
	if msg := validateName(req.Name); msg != "" {
		return msg
	}
	if len(req.ServiceClassPath) < 1 || len(req.ServiceClassPath) > 255 {
		return "serviceClassPath must be 1-255 characters"
	}
	if !validLBServiceClassPaths[req.ServiceClassPath] {
		return fmt.Sprintf("invalid serviceClassPath: %q", req.ServiceClassPath)
	}
	if len(req.NameServers) > 0 && (len(req.NameServers) < 1 || len(req.NameServers) > 3) {
		return "nameServers must have 1-3 entries"
	}
	if len(req.Interfaces) < 1 || len(req.Interfaces) > 5 {
		return "interfaces must have 1-5 entries"
	}
	for _, iface := range req.Interfaces {
		if len(iface.IpPool) > 20 {
			return "maximum 20 IP ranges per interface"
		}
		if iface.NetmaskLen != nil && (*iface.NetmaskLen < 8 || *iface.NetmaskLen > 29) {
			return "netmaskLen must be between 8 and 29"
		}
	}
	return ""
}

func validateCreateCertificate(req *createCertificateReq) string {
	if msg := validateCertName(req.Name); msg != "" {
		return msg
	}
	if len(req.CertificatePem) == 0 || len(req.CertificatePem) > 1000000 {
		return "certificatePem must be 1-1000000 characters"
	}
	if len(req.PrivatekeyPem) == 0 || len(req.PrivatekeyPem) > 1000000 {
		return "privatekeyPem must be 1-1000000 characters"
	}
	if req.IntermediateCertificatePem != nil && len(*req.IntermediateCertificatePem) > 1000000 {
		return "intermediateCertificatePem must be at most 1000000 characters"
	}
	return ""
}

func validateName(name string) string {
	if len(name) < 1 || len(name) > 20 {
		return "name must be 1-20 characters"
	}
	if !namePattern.MatchString(name) {
		return "name must contain only alphanumeric characters, underscores, and hyphens"
	}
	return ""
}

func validateCertName(name string) string {
	if len(name) < 1 || len(name) > 20 {
		return "name must be 1-20 characters"
	}
	if !certNamePattern.MatchString(name) {
		return "name must contain only alphanumeric characters, underscores, hyphens, and dots"
	}
	return ""
}
