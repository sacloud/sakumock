package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubServiceConfig struct {
	name string
	env  []EnvVar
}

func (c stubServiceConfig) Name() string                            { return c.name }
func (c stubServiceConfig) ListenAddr() string                      { return "127.0.0.1:18999" }
func (c stubServiceConfig) ClientEnv() []EnvVar                     { return c.env }
func (c stubServiceConfig) NewServer(ServerOptions) (Server, error) { return nil, nil }
func (c stubServiceConfig) Doc() string                             { return "" }

func TestMountedClientEnv(t *testing.T) {
	cfg := stubServiceConfig{name: "svc", env: []EnvVar{
		{Key: "SAKURA_ENDPOINTS_A", Value: "http://127.0.0.1:18999"},
		{Key: "SAKURA_ENDPOINTS_B", Value: "http://127.0.0.1:18999/"},
	}}
	got, err := MountedClientEnv(cfg, "0.0.0.0:18000")
	if err != nil {
		t.Fatal(err)
	}
	want := []EnvVar{
		{Key: "SAKURA_ENDPOINTS_A", Value: "http://0.0.0.0:18000/svc"},
		{Key: "SAKURA_ENDPOINTS_B", Value: "http://0.0.0.0:18000/svc/"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %+v, want %+v", got[i], want[i])
		}
	}
}

func TestNewMountHandler(t *testing.T) {
	echo := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, name+" "+r.URL.Path)
		})
	}
	h := NewMountHandler([]MountedHandler{
		{Name: "a", Handler: echo("a")},
		{Name: "a-b", Handler: echo("a-b")},
	})
	tests := []struct {
		path, want string
		status     int
	}{
		{"/a/items/1", "a /items/1", http.StatusOK},
		{"/a-b/items/1", "a-b /items/1", http.StatusOK},
		{"/items/1", "", http.StatusNotFound},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if rec.Code != tt.status {
			t.Errorf("GET %s: status %d, want %d", tt.path, rec.Code, tt.status)
			continue
		}
		if tt.want != "" && rec.Body.String() != tt.want {
			t.Errorf("GET %s: body %q, want %q", tt.path, rec.Body.String(), tt.want)
		}
	}
}
