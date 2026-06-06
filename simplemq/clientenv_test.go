package simplemq

import "testing"

func TestClientEnv(t *testing.T) {
	env := Config{Addr: "127.0.0.1:18080"}.ClientEnv()
	want := map[string]string{
		"SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE":   "http://127.0.0.1:18080",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE": "http://127.0.0.1:18080",
	}
	if len(env) != len(want) {
		t.Fatalf("got %d vars, want %d: %+v", len(env), len(want), env)
	}
	for _, e := range env {
		if want[e.Key] != e.Value {
			t.Errorf("%s = %q, want %q", e.Key, e.Value, want[e.Key])
		}
	}
}
