package main

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sacloud/sacloud-sdk-go/common/saclient"
)

func newTestEnvCmd() *EnvCmd {
	c := &EnvCmd{}
	c.Addr = "127.0.0.1:18000"
	c.Simplemq.Addr = "127.0.0.1:18080"
	c.Kms.Addr = "127.0.0.1:18081"
	c.Secretmanager.Addr = "127.0.0.1:18082"
	c.Simplenotification.Addr = "127.0.0.1:18083"
	c.Monitoringsuite.Addr = "127.0.0.1:18084"
	c.Eventbus.Addr = "127.0.0.1:18085"
	c.Objectstorage.Addr = "127.0.0.1:18086"
	return c
}

func TestEnvCmdDefault(t *testing.T) {
	c := newTestEnvCmd()
	vars, err := c.clientEnv()
	if err != nil {
		t.Fatalf("clientEnv: %v", err)
	}
	rendered := strings.Join(envLines(vars), "\n")
	// Every service shares --addr, mounted under /<service>.
	for _, want := range []string{
		"SAKURA_ENDPOINTS_KMS=http://127.0.0.1:18000/kms",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE=http://127.0.0.1:18000/simplemq",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://127.0.0.1:18000/simplemq",
		"SAKURA_ENDPOINTS_EVENTBUS=http://127.0.0.1:18000/eventbus/", // trailing slash: see eventbus.Config.ClientEnv
		"SAKURA_ENDPOINTS_APPRUN_DEDICATED=http://127.0.0.1:18000/apprun-dedicated",
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("env missing %q\n%s", want, rendered)
		}
	}
}

func TestEnvCmdPerServicePorts(t *testing.T) {
	c := newTestEnvCmd()
	c.PerServicePorts = true
	vars, err := c.clientEnv()
	if err != nil {
		t.Fatalf("clientEnv: %v", err)
	}
	rendered := strings.Join(envLines(vars), "\n")
	for _, want := range []string{
		"SAKURA_ENDPOINTS_KMS=http://127.0.0.1:18081",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE=http://127.0.0.1:18080",
		"SAKURA_ENDPOINTS_EVENTBUS=http://127.0.0.1:18085/", // trailing slash: see eventbus.Config.ClientEnv
		"SAKURA_ENDPOINTS_OBJECT_STORAGE=http://127.0.0.1:18086",
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("env missing %q\n%s", want, rendered)
		}
	}
	// The data plane is off by default, so no AWS_* vars are emitted.
	if strings.Contains(rendered, "AWS_") {
		t.Errorf("unexpected AWS_* vars while data plane disabled\n%s", rendered)
	}
}

func TestEnvCmdDataPlane(t *testing.T) {
	c := newTestEnvCmd()
	c.Objectstorage.EnableDataPlane = true
	c.Objectstorage.DataPlaneAddr = "127.0.0.1:28086"
	c.Objectstorage.DataPlaneRegion = "jp-north-1"

	vars, err := c.clientEnv()
	if err != nil {
		t.Fatalf("clientEnv: %v", err)
	}
	rendered := strings.Join(envLines(vars), "\n")
	for _, want := range []string{
		"AWS_ENDPOINT_URL_S3=http://127.0.0.1:28086",
		"AWS_ACCESS_KEY_ID=sakumock",
		"AWS_SECRET_ACCESS_KEY=sakumocksecret",
		"AWS_DEFAULT_REGION=jp-north-1",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("data plane env missing %q\n%s", want, rendered)
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
	// Host is rewritten; the port and the mount path are kept.
	for _, want := range []string{
		"SAKURA_ENDPOINTS_KMS=http://sakumock:18000/kms",
		"SAKURA_ENDPOINTS_EVENTBUS=http://sakumock:18000/eventbus/",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("env missing %q\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "127.0.0.1") {
		t.Errorf("--host should have replaced every endpoint host\n%s", rendered)
	}
}

func TestEnvCmdHostRewritePerServicePorts(t *testing.T) {
	c := newTestEnvCmd()
	c.PerServicePorts = true
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
		"SAKURA_ENDPOINTS_EVENTBUS=http://sakumock:18085/", // trailing slash survives the host rewrite
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

// The SDK must resolve the same endpoints from the profile --profile-dir writes
// as from the per-service variables it replaces.
func TestEnvCmdProfileDir(t *testing.T) {
	c := newTestEnvCmd()
	c.Objectstorage.EnableDataPlane = true
	c.ProfileDir = t.TempDir()
	c.Output = filepath.Join(t.TempDir(), "sakumock.env")
	if err := c.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	dotenv, err := os.ReadFile(c.Output)
	if err != nil {
		t.Fatal(err)
	}
	var env []string
	for line := range strings.SplitSeq(string(dotenv), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			env = append(env, line)
		}
	}
	rendered := strings.Join(env, "\n")
	for _, want := range []string{
		"SAKURA_PROFILE=sakumock",
		"SAKURA_PROFILE_DIR=" + c.ProfileDir,
		"AWS_ACCESS_KEY_ID=sakumock", // not expressible in a profile, so it stays
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("env missing %q\n%s", want, rendered)
		}
	}
	for _, unwanted := range []string{"SAKURA_ENDPOINTS_", "SAKURA_ACCESS_TOKEN"} {
		if strings.Contains(rendered, unwanted) {
			t.Errorf("%s* should be in the profile, not the env\n%s", unwanted, rendered)
		}
	}

	var sa saclient.Client
	if err := sa.SetEnviron(env); err != nil {
		t.Fatal(err)
	}
	got, err := sa.EndpointConfig()
	if err != nil {
		t.Fatalf("EndpointConfig: %v", err)
	}
	vars, err := c.clientEnv()
	if err != nil {
		t.Fatal(err)
	}
	var want saclient.Client
	if err := want.SetEnviron(envLines(vars)); err != nil {
		t.Fatal(err)
	}
	wantCfg, err := want.EndpointConfig()
	if err != nil {
		t.Fatalf("EndpointConfig: %v", err)
	}
	if len(wantCfg.Endpoints) == 0 {
		t.Fatal("no endpoints resolved from the per-service variables")
	}
	if !maps.Equal(got.Endpoints, wantCfg.Endpoints) {
		t.Errorf("endpoints from the profile = %v\nwant %v", got.Endpoints, wantCfg.Endpoints)
	}
}
