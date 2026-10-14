package main

import (
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
