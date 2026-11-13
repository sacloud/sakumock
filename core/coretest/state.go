// Package coretest holds test helpers shared by every service's tests.
package coretest

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sacloud/sakumock/core"
)

// StateRestart checks a service's --state-dir support by running it twice on
// one state directory: populate creates resources against the first run, and
// verify checks them against the second, each given the server's base URL.
//
// Beyond what verify asserts, it fails when the state file is not on disk
// before the first run shuts down (the save after each request is missing),
// when either run records an OpenAPI spec violation, or when the second run
// leaves a different state file although verify changed nothing (restoring
// loses or reorders something). A verify that does change state must pass
// mutates=true to skip that last check.
//
// The service is built through cfg.NewServer, as the unified binary does, so
// the test needs no access to the service's unexported configuration.
func StateRestart(t *testing.T, cfg core.ServiceConfig, populate, verify func(t *testing.T, url string), mutates ...bool) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, cfg.Name()+".json")

	run := func(fn func(t *testing.T, url string), afterRequests func()) {
		t.Helper()
		srv, err := cfg.NewServer(core.ServerOptions{StateDir: dir})
		if err != nil {
			t.Fatalf("start %s on %s: %v", cfg.Name(), dir, err)
		}
		ts := httptest.NewServer(srv)
		fn(t, ts.URL)
		ts.Close()
		if afterRequests != nil {
			afterRequests()
		}
		if v, ok := srv.(interface{ SpecViolations() []core.SpecViolation }); ok {
			if violations := v.SpecViolations(); len(violations) != 0 {
				t.Errorf("spec violations: %+v", violations)
			}
		}
		srv.Close()
	}

	run(populate, func() {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("state file not written before shutdown: %v", err)
		}
	})
	if t.Failed() {
		return
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	run(verify, nil)
	if len(mutates) > 0 && mutates[0] {
		return
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, again) {
		t.Errorf("state file changed across a restart without a change of state:\n--- first run\n%s\n--- second run\n%s", saved, again)
	}
}
