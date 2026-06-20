package apprundedicated_test

import (
	"testing"

	"github.com/sacloud/sakumock/apprundedicated"
)

func TestClientEnv(t *testing.T) {
	cfg := apprundedicated.Config{Addr: "127.0.0.1:18089"}
	env := cfg.ClientEnv()
	if len(env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(env))
	}
	if env[0].Key != "SAKURA_ENDPOINTS_APPRUN_DEDICATED" {
		t.Fatalf("unexpected key: %s", env[0].Key)
	}
	if env[0].Value != "http://127.0.0.1:18089" {
		t.Fatalf("unexpected value: %s", env[0].Value)
	}
}
