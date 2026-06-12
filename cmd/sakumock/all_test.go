package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func newTestAllCmd() *AllCmd {
	c := &AllCmd{}
	c.Simplemq.Addr = "127.0.0.1:18080"
	c.Kms.Addr = "127.0.0.1:18081"
	c.Secretmanager.Addr = "127.0.0.1:18082"
	c.Simplenotification.Addr = "127.0.0.1:18083"
	c.Monitoringsuite.Addr = "127.0.0.1:18084"
	c.Eventbus.Addr = "127.0.0.1:18085"
	return c
}

func TestAllBuild(t *testing.T) {
	services, err := newTestAllCmd().build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer func() {
		for _, s := range services {
			s.server.Close()
		}
	}()

	if got, want := len(services), 6; got != want {
		t.Fatalf("got %d services, want %d", got, want)
	}
	if services[0].cfg.Name() != "simplemq" || len(services[0].cfg.ClientEnv()) != 2 {
		t.Errorf("simplemq should expose both control- and data-plane keys, got %+v", services[0])
	}
}

func TestAllSharesOneIDGenerator(t *testing.T) {
	c := newTestAllCmd()
	services, err := c.build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer func() {
		for _, s := range services {
			s.server.Close()
		}
	}()

	byName := map[string]core.Server{}
	for _, s := range services {
		byName[s.cfg.Name()] = s.server
	}

	// Create one resource in two different services (both auth-free). With a
	// shared generator their IDs must differ; without it both would be the
	// same base value.
	vaultID := postResourceID(t, byName["secretmanager"], "/secretmanager/vaults",
		`{"Vault":{"Name":"v","KmsKeyID":"k"}}`, "Vault")
	destID := postResourceID(t, byName["simplenotification"], "/commonserviceitem",
		`{"CommonServiceItem":{"Name":"d","Provider":{"Class":"saknoticedestination"}}}`, "CommonServiceItem")

	if vaultID == "" || destID == "" {
		t.Fatalf("missing IDs: vault=%q dest=%q", vaultID, destID)
	}
	if vaultID == destID {
		t.Errorf("services minted the same ID %q; under `all` they should share one generator and differ", vaultID)
	}
}

// postResourceID POSTs body to h and returns the ID from the {wrapper:{ID}} response.
func postResourceID(t *testing.T, h core.Server, path, body, wrapper string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST %s: status %d, body %s", path, rec.Code, rec.Body.String())
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	var item struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(wrapped[wrapper], &item); err != nil {
		t.Fatalf("decode %s.%s: %v", path, wrapper, err)
	}
	return item.ID
}

func envLines(vars []core.EnvVar) []string {
	lines := make([]string, len(vars))
	for i, v := range vars {
		lines[i] = v.Key + "=" + v.Value
	}
	return lines
}
