package apprundedicated

import (
	"strings"
	"testing"
)

func TestValidateCreateCluster(t *testing.T) {
	valid := &createClusterReq{
		Name:               "my-cluster",
		ServicePrincipalID: "123456789012",
		Ports:              []clusterPortJSON{{Port: 443, Protocol: "https"}},
	}
	if msg := validateCreateCluster(valid); msg != "" {
		t.Fatalf("expected valid, got %q", msg)
	}

	tests := []struct {
		name   string
		modify func(r *createClusterReq)
		want   string
	}{
		{"empty name", func(r *createClusterReq) { r.Name = "" }, "name must be 1-20 characters"},
		{"name too long", func(r *createClusterReq) { r.Name = strings.Repeat("a", 21) }, "name must be 1-20 characters"},
		{"name invalid chars", func(r *createClusterReq) { r.Name = "my cluster!" }, "alphanumeric"},
		{"short spID", func(r *createClusterReq) { r.ServicePrincipalID = "123" }, "servicePrincipalID must be exactly 12"},
		{"no ports", func(r *createClusterReq) { r.Ports = nil }, "at least one port"},
		{"too many ports", func(r *createClusterReq) {
			r.Ports = make([]clusterPortJSON, 6)
			for i := range r.Ports {
				r.Ports[i] = clusterPortJSON{Port: uint16(8080 + i), Protocol: "http"}
			}
		}, "maximum 5 ports"},
		{"reserved port", func(r *createClusterReq) {
			r.Ports = []clusterPortJSON{{Port: 5955, Protocol: "tcp"}}
		}, "reserved"},
		{"invalid protocol", func(r *createClusterReq) {
			r.Ports = []clusterPortJSON{{Port: 80, Protocol: "udp"}}
		}, "protocol must be one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			r.Ports = append([]clusterPortJSON{}, valid.Ports...)
			tt.modify(&r)
			msg := validateCreateCluster(&r)
			if msg == "" {
				t.Fatal("expected error, got none")
			}
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected error containing %q, got %q", tt.want, msg)
			}
		})
	}
}

func TestValidateCreateApplication(t *testing.T) {
	valid := &createApplicationReq{Name: "my-app", ClusterID: "some-uuid"}
	if msg := validateCreateApplication(valid); msg != "" {
		t.Fatalf("expected valid, got %q", msg)
	}

	tests := []struct {
		name   string
		modify func(r *createApplicationReq)
		want   string
	}{
		{"empty name", func(r *createApplicationReq) { r.Name = "" }, "name must be 1-20"},
		{"name too long", func(r *createApplicationReq) { r.Name = strings.Repeat("x", 21) }, "name must be 1-20"},
		{"empty clusterID", func(r *createApplicationReq) { r.ClusterID = "" }, "clusterID is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			tt.modify(&r)
			msg := validateCreateApplication(&r)
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, msg)
			}
		})
	}
}

func TestValidateCreateVersion(t *testing.T) {
	fixedScale := int32(1)
	valid := &createVersionReq{
		CPU:         1000,
		Memory:      2048,
		ScalingMode: "manual",
		FixedScale:  &fixedScale,
		Image:       "nginx:latest",
	}
	if msg := validateCreateVersion(valid); msg != "" {
		t.Fatalf("expected valid, got %q", msg)
	}

	tests := []struct {
		name   string
		modify func(r *createVersionReq)
		want   string
	}{
		{"cpu too low", func(r *createVersionReq) { r.CPU = 50 }, "cpu must be between"},
		{"cpu too high", func(r *createVersionReq) { r.CPU = 65000 }, "cpu must be between"},
		{"memory too low", func(r *createVersionReq) { r.Memory = 64 }, "memory must be between"},
		{"memory too high", func(r *createVersionReq) { r.Memory = 200000 }, "memory must be between"},
		{"invalid scaling mode", func(r *createVersionReq) { r.ScalingMode = "auto" }, "scalingMode must be one of"},
		{"fixedScale too high", func(r *createVersionReq) {
			v := int32(51)
			r.FixedScale = &v
		}, "fixedScale must be between"},
		{"minScale > maxScale", func(r *createVersionReq) {
			min, max := int32(5), int32(3)
			r.MinScale = &min
			r.MaxScale = &max
		}, "minScale must be less than"},
		{"scaleInThreshold too low", func(r *createVersionReq) {
			v := int32(20)
			r.ScaleInThreshold = &v
		}, "scaleInThreshold must be between"},
		{"scaleOutThreshold too high", func(r *createVersionReq) {
			v := int32(100)
			r.ScaleOutThreshold = &v
		}, "scaleOutThreshold must be between"},
		{"empty image", func(r *createVersionReq) { r.Image = "" }, "image must be 1-512"},
		{"image too long", func(r *createVersionReq) { r.Image = strings.Repeat("a", 513) }, "image must be 1-512"},
		{"too many cmd", func(r *createVersionReq) { r.Cmd = make([]string, 21) }, "maximum 20 cmd"},
		{"too many exposed ports", func(r *createVersionReq) {
			r.ExposedPorts = make([]exposedPortJSON, 6)
		}, "maximum 5 exposed ports"},
		{"too many env vars", func(r *createVersionReq) {
			r.Env = make([]createEnvJSON, 51)
			for i := range r.Env {
				r.Env[i].Key = "K"
			}
		}, "maximum 50 environment"},
		{"env key empty", func(r *createVersionReq) {
			r.Env = []createEnvJSON{{Key: ""}}
		}, "environment variable key must be 1-255"},
		{"env value too long", func(r *createVersionReq) {
			v := strings.Repeat("x", 4097)
			r.Env = []createEnvJSON{{Key: "K", Value: &v}}
		}, "environment variable value must be at most 4096"},
		{"healthCheck intervalSeconds too low", func(r *createVersionReq) {
			r.ExposedPorts = []exposedPortJSON{{
				TargetPort:  80,
				HealthCheck: &healthCheckJSON{Path: "/", IntervalSeconds: 1, TimeoutSeconds: 5},
			}}
		}, "healthCheck.intervalSeconds must be between 3 and 60"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			tt.modify(&r)
			msg := validateCreateVersion(&r)
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, msg)
			}
		})
	}
}

func TestValidateCreateASG(t *testing.T) {
	valid := &createASGReq{
		Name:                   "my-asg",
		Zone:                   "is1a",
		WorkerServiceClassPath: "cloud/apprun/dedicated/worker/1vcpu_2gb",
		MinNodes:               1,
		MaxNodes:               3,
		NameServers:            []string{"210.188.224.10"},
		Interfaces: []asgInterfaceJSON{
			{InterfaceIndex: 0, Upstream: "shared"},
		},
	}
	if msg := validateCreateASG(valid); msg != "" {
		t.Fatalf("expected valid, got %q", msg)
	}

	tests := []struct {
		name   string
		modify func(r *createASGReq)
		want   string
	}{
		{"invalid name", func(r *createASGReq) { r.Name = "bad name!" }, "alphanumeric"},
		{"empty zone", func(r *createASGReq) { r.Zone = "" }, "zone is required"},
		{"invalid service class", func(r *createASGReq) {
			r.WorkerServiceClassPath = "cloud/apprun/dedicated/worker/99vcpu_999gb"
		}, "invalid workerServiceClassPath"},
		{"minNodes < 1", func(r *createASGReq) { r.MinNodes = 0 }, "minNodes must be between"},
		{"maxNodes > 10", func(r *createASGReq) { r.MaxNodes = 11 }, "maxNodes must be between"},
		{"min > max", func(r *createASGReq) { r.MinNodes = 5; r.MaxNodes = 3 }, "minNodes must be less"},
		{"no interfaces", func(r *createASGReq) { r.Interfaces = nil }, "interfaces must have 1-5"},
		{"too many interfaces", func(r *createASGReq) {
			r.Interfaces = make([]asgInterfaceJSON, 6)
		}, "interfaces must have 1-5"},
		{"ipPool too large", func(r *createASGReq) {
			r.Interfaces[0].IpPool = make([]ipRangeJSON, 21)
		}, "maximum 20 IP ranges"},
		{"netmaskLen too low", func(r *createASGReq) {
			v := int16(5)
			r.Interfaces[0].NetmaskLen = &v
		}, "netmaskLen must be between"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			r.Interfaces = append([]asgInterfaceJSON{}, valid.Interfaces...)
			tt.modify(&r)
			msg := validateCreateASG(&r)
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, msg)
			}
		})
	}
}

func TestValidateCreateLB(t *testing.T) {
	valid := &createLBReq{
		Name:             "my-lb",
		ServiceClassPath: "cloud/apprun/dedicated/lb/1vcpu_2gb",
		NameServers:      []string{"210.188.224.10"},
		Interfaces: []lbInterfaceJSON{
			{InterfaceIndex: 0, Upstream: "shared"},
		},
	}
	if msg := validateCreateLB(valid); msg != "" {
		t.Fatalf("expected valid, got %q", msg)
	}

	tests := []struct {
		name   string
		modify func(r *createLBReq)
		want   string
	}{
		{"invalid name", func(r *createLBReq) { r.Name = "" }, "name must be 1-20"},
		{"invalid service class", func(r *createLBReq) {
			r.ServiceClassPath = "cloud/invalid/path"
		}, "invalid serviceClassPath"},
		{"no interfaces", func(r *createLBReq) { r.Interfaces = nil }, "interfaces must have 1-5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			r.Interfaces = append([]lbInterfaceJSON{}, valid.Interfaces...)
			tt.modify(&r)
			msg := validateCreateLB(&r)
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, msg)
			}
		})
	}
}

func TestValidateCreateCertificate(t *testing.T) {
	valid := &createCertificateReq{
		Name:           "my-cert",
		CertificatePem: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
	}
	if msg := validateCreateCertificate(valid); msg != "" {
		t.Fatalf("expected valid, got %q", msg)
	}

	tests := []struct {
		name   string
		modify func(r *createCertificateReq)
		want   string
	}{
		{"empty name", func(r *createCertificateReq) { r.Name = "" }, "name must be 1-20"},
		{"cert name with dot (allowed)", func(r *createCertificateReq) { r.Name = "my.cert" }, ""},
		{"cert name with space (rejected)", func(r *createCertificateReq) { r.Name = "my cert" }, "alphanumeric"},
		{"empty certificatePem", func(r *createCertificateReq) { r.CertificatePem = "" }, "certificatePem must be 1-1000000"},
		{"empty privatekeyPem", func(r *createCertificateReq) { r.PrivatekeyPem = "" }, "privatekeyPem must be 1-1000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			tt.modify(&r)
			msg := validateCreateCertificate(&r)
			if tt.want == "" {
				if msg != "" {
					t.Fatalf("expected valid, got %q", msg)
				}
				return
			}
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, msg)
			}
		})
	}
}
