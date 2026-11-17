package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sacloud/sakumock/core"
)

func TestHealthcheck(t *testing.T) {
	healthy := httptest.NewServer(core.NewMountHandler(nil))
	defer healthy.Close()
	unhealthy := httptest.NewServer(http.NotFoundHandler())
	defer unhealthy.Close()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"healthy", healthy.URL + core.HealthPath, false},
		{"non-200", unhealthy.URL + core.HealthPath, true},
		{"unreachable", "http://127.0.0.1:1" + core.HealthPath, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &HealthcheckCmd{URL: tt.url, Timeout: time.Second}
			if err := c.Run(t.Context()); (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --ca-cert makes the probe trust a --tls-auto sakumock's CA.
func TestHealthcheckCACert(t *testing.T) {
	auto := core.TLSAuto{Auto: true, CADir: t.TempDir()}
	files, cleanup, err := auto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: core.NewMountHandler(nil)}
	go core.ServeListener(srv, ln, files)
	defer srv.Close()

	url := "https://" + ln.Addr().String() + core.HealthPath
	c := &HealthcheckCmd{URL: url, Timeout: time.Second, CACert: auto.CACertPath()}
	if err := c.Run(t.Context()); err != nil {
		t.Errorf("Run() with --ca-cert: %v", err)
	}
	c = &HealthcheckCmd{URL: url, Timeout: time.Second}
	if err := c.Run(t.Context()); err == nil {
		t.Error("Run() without --ca-cert trusted the generated certificate")
	}
}
