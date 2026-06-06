package monitoringsuite

import "testing"

func TestClientEnv(t *testing.T) {
	env := Config{Addr: "127.0.0.1:18084"}.ClientEnv()
	if len(env) != 1 {
		t.Fatalf("got %d vars, want 1: %+v", len(env), env)
	}
	if env[0].Key != "SAKURA_ENDPOINTS_MONITORING_SUITE" || env[0].Value != "http://127.0.0.1:18084" {
		t.Errorf("unexpected env var: %+v", env[0])
	}
}
