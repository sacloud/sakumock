package monitoringsuite_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/monitoringsuite"
)

// TestRequestLogIncludesServiceName verifies that, with no logger injected, the
// server tags request logs with its service name via the default logger.
func TestRequestLogIncludesServiceName(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	srv := monitoringsuite.NewTestServer(monitoringsuite.Config{})
	defer srv.Close()

	resp, err := http.Get(srv.TestURL() + "/anything")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	out := buf.String()
	if !strings.Contains(out, `"msg":"request"`) {
		t.Fatalf("expected a request log line, got: %s", out)
	}
	if !strings.Contains(out, `"service":"monitoringsuite"`) {
		t.Fatalf("expected service=monitoringsuite in log, got: %s", out)
	}
}

// TestServerOptionsLoggerInjected verifies that a logger injected through
// core.ServerOptions (as the unified binary does) receives the service-tagged
// request logs, rather than the global default.
func TestServerOptionsLoggerInjected(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	srv, err := monitoringsuite.Config{}.NewServer(core.ServerOptions{Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/anything")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if out := buf.String(); !strings.Contains(out, `"service":"monitoringsuite"`) {
		t.Fatalf("expected injected logger to receive service=monitoringsuite, got: %s", out)
	}
}
