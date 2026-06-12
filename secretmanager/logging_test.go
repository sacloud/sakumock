package secretmanager_test

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/sacloud/sacloud-sdk-go/api/secretmanager/apis/v1"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/secretmanager"
)

// TestStoreLogIncludesServiceName verifies that a logger injected through
// core.ServerOptions reaches the store, so store-level operation logs (here the
// "vault created" line) are tagged with the service name.
func TestStoreLogIncludesServiceName(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	srv, err := secretmanager.Config{}.NewServer(core.ServerOptions{Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Creating a secret lazily creates its vault, emitting a store log line.
	op := newTestSecretOp(t, ts.URL, "vault-logging")
	if _, err := op.Create(t.Context(), v1.CreateSecret{Name: "foo", Value: "bar"}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, `"msg":"vault created"`) {
		t.Fatalf("expected a 'vault created' store log line, got: %s", out)
	}
	if !strings.Contains(out, `"service":"secretmanager"`) {
		t.Fatalf("expected service=secretmanager on store log, got: %s", out)
	}
}
