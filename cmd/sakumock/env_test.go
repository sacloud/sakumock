package main

import (
	"strings"
	"testing"
)

func newTestEnvCmd() *EnvCmd {
	c := &EnvCmd{}
	c.Simplemq.Addr = "127.0.0.1:18080"
	c.Kms.Addr = "127.0.0.1:18081"
	c.Secretmanager.Addr = "127.0.0.1:18082"
	c.Simplenotification.Addr = "127.0.0.1:18083"
	c.Monitoringsuite.Addr = "127.0.0.1:18084"
	return c
}

func TestEnvCmdDefaultHost(t *testing.T) {
	c := newTestEnvCmd()
	vars, err := c.clientEnv()
	if err != nil {
		t.Fatalf("clientEnv: %v", err)
	}
	rendered := strings.Join(envLines(vars), "\n")
	for _, want := range []string{
		"SAKURA_ENDPOINTS_KMS=http://127.0.0.1:18081",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE=http://127.0.0.1:18080",
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("env missing %q\n%s", want, rendered)
		}
	}
}

func TestEnvCmdHostRewrite(t *testing.T) {
	c := newTestEnvCmd()
	c.Host = "sakumock"
	vars, err := c.clientEnv()
	if err != nil {
		t.Fatalf("clientEnv: %v", err)
	}
	rendered := strings.Join(envLines(vars), "\n")
	// Host is rewritten, port is kept.
	for _, want := range []string{
		"SAKURA_ENDPOINTS_KMS=http://sakumock:18081",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://sakumock:18080",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("env missing %q\n%s", want, rendered)
		}
	}
	// Credentials are not URLs and must be left untouched.
	if !strings.Contains(rendered, "SAKURA_ACCESS_TOKEN=dummy") {
		t.Errorf("credentials should be unchanged by --host\n%s", rendered)
	}
	if strings.Contains(rendered, "127.0.0.1") {
		t.Errorf("--host should have replaced every endpoint host\n%s", rendered)
	}
}

func TestWithHost(t *testing.T) {
	tests := []struct {
		in, host, want string
	}{
		{"http://127.0.0.1:18081", "localhost", "http://localhost:18081"},
		{"http://0.0.0.0:28080", "sakumock", "http://sakumock:28080"},
		{"http://127.0.0.1", "localhost", "http://localhost"}, // port-less
	}
	for _, tt := range tests {
		got, err := withHost(tt.in, tt.host)
		if err != nil {
			t.Errorf("withHost(%q, %q): %v", tt.in, tt.host, err)
			continue
		}
		if got != tt.want {
			t.Errorf("withHost(%q, %q) = %q, want %q", tt.in, tt.host, got, tt.want)
		}
	}
}

func TestAllBindAddr(t *testing.T) {
	c := &AllCmd{}
	if got := c.bindAddr("127.0.0.1:18081"); got != "127.0.0.1:18081" {
		t.Errorf("no --listen-host: got %q, want unchanged", got)
	}
	c.ListenHost = "0.0.0.0"
	if got := c.bindAddr("127.0.0.1:18081"); got != "0.0.0.0:18081" {
		t.Errorf("--listen-host 0.0.0.0: got %q, want 0.0.0.0:18081", got)
	}
}
