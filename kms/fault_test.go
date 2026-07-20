package kms_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/kms"
)

func TestFaultDisabledByDefault(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()

	resp, err := http.Get(srv.TestURL() + "/kms/keys")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFaultStatusInjected(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{Fault: []string{"503:1"}})
	defer srv.Close()

	resp, err := http.Get(srv.TestURL() + "/kms/keys")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	// The body must be the KMS error envelope, not the generic default.
	var envelope struct {
		Error string `json:"error"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == "" {
		t.Fatalf("expected KMS error envelope, got %q (err=%v)", body, err)
	}
	if !strings.Contains(envelope.Error, "fault injection") {
		t.Errorf("unexpected error message: %q", envelope.Error)
	}
}

func TestFaultStatusAfter(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{Fault: []string{"503:1:after"}})
	defer srv.Close()

	resp, err := http.Post(srv.TestURL()+"/kms/keys", "application/json",
		strings.NewReader(`{"Key":{"Name":"k1","KeyOrigin":"generated"}}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "replaced status") {
		t.Errorf("expected replaced-status message, got %q", body)
	}
}

func TestFaultReset(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{Fault: []string{"reset:1"}})
	defer srv.Close()

	if _, err := http.Get(srv.TestURL() + "/kms/keys"); err == nil {
		t.Fatal("expected a transport error, got response")
	}
}

func TestFaultInvalidSpec(t *testing.T) {
	if _, err := kms.NewHandler(kms.Config{Fault: []string{"bogus"}}); err == nil {
		t.Fatal("expected NewHandler to fail on an invalid fault spec")
	}
}
