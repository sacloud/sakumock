package apprundedicated

import (
	"strings"
	"testing"
)

// These tests cover only the domain rules kept in validate.go; the
// spec-expressible constraints (required, lengths, patterns, ranges, enums)
// are enforced by the generated bodySchemas table via core.BodyValidator.

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
		{"no ports", func(r *createClusterReq) { r.Ports = nil }, "at least one port"},
		{"reserved port", func(r *createClusterReq) {
			r.Ports = []clusterPortJSON{{Port: 5955, Protocol: "tcp"}}
		}, "reserved"},
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
		{"minScale > maxScale", func(r *createVersionReq) {
			min, max := int32(5), int32(3)
			r.MinScale = &min
			r.MaxScale = &max
		}, "minScale must be less than"},
		{"empty image", func(r *createVersionReq) { r.Image = "" }, "image must be 1-512"},
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
		{"empty zone", func(r *createASGReq) { r.Zone = "" }, "zone is required"},
		{"invalid service class", func(r *createASGReq) {
			r.WorkerServiceClassPath = "cloud/apprun/dedicated/worker/99vcpu_999gb"
		}, "invalid workerServiceClassPath"},
		{"min > max", func(r *createASGReq) { r.MinNodes = 5; r.MaxNodes = 3 }, "minNodes must be less"},
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

	if msg := validateCreateLB(&createLBReq{
		Name:             "my-lb",
		ServiceClassPath: "cloud/invalid/path",
		Interfaces:       valid.Interfaces,
	}); !strings.Contains(msg, "invalid serviceClassPath") {
		t.Fatalf("expected invalid serviceClassPath, got %q", msg)
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
		{"empty certificatePem", func(r *createCertificateReq) { r.CertificatePem = "" }, "certificatePem must be 1-1000000"},
		{"empty privatekeyPem", func(r *createCertificateReq) { r.PrivatekeyPem = "" }, "privatekeyPem must be 1-1000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := *valid
			tt.modify(&r)
			msg := validateCreateCertificate(&r)
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, msg)
			}
		})
	}
}
