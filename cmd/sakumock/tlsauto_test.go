package main

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	kmssdk "github.com/sacloud/sacloud-sdk-go/api/kms"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/core"
)

// An SDK client configured only from `sakumock env` reaches a --tls-auto
// sakumock: the generated CA is trusted through the dotenv, and through the
// profile with --profile-dir.
func TestTLSAutoClientEnv(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	auto := core.TLSAuto{Auto: true, CADir: t.TempDir()}

	all := newTestAllCmd()
	all.TLSAuto = auto
	files, cleanup, err := all.TLSAuto.Generate(all.ListenHost)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer cleanup()
	all.TLS = files
	instances, err := all.build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer func() {
		for _, i := range instances {
			i.server.Close()
		}
	}()
	srv := &http.Server{Handler: mountHandler(instances)}
	go core.ServeListener(srv, ln, all.TLS)
	defer srv.Shutdown(context.Background())

	listKeys := func(t *testing.T, env []string) error {
		t.Helper()
		var sa saclient.Client
		if err := sa.SetEnviron(env); err != nil {
			t.Fatal(err)
		}
		client, err := kmssdk.NewClient(&sa)
		if err != nil {
			t.Fatal(err)
		}
		_, err = kmssdk.NewKeyOp(client).List(t.Context(), nil, nil)
		return err
	}

	env := newTestEnvCmd()
	env.Addr = ln.Addr().String()
	env.TLSAuto = auto
	vars, err := env.clientEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("dotenv", func(t *testing.T) {
		if err := listKeys(t, envLines(vars)); err != nil {
			t.Errorf("SDK call with the emitted env: %v", err)
		}
	})
	t.Run("profile", func(t *testing.T) {
		profileEnv, err := core.WriteClientProfile(filepath.Join(t.TempDir(), "profiles"), vars)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range profileEnv {
			if e.Key == core.CACertPathEnv {
				t.Errorf("%s should be in the profile, not the env", e.Key)
			}
		}
		if err := listKeys(t, envLines(profileEnv)); err != nil {
			t.Errorf("SDK call with the emitted profile: %v", err)
		}
	})
	t.Run("untrusted without the CA", func(t *testing.T) {
		var without []core.EnvVar
		for _, e := range vars {
			if e.Key != core.CACertPathEnv {
				without = append(without, e)
			}
		}
		if err := listKeys(t, envLines(without)); err == nil {
			t.Error("SDK call succeeded without trusting the generated CA")
		}
	})
}

// Forwarding SDK clients of service link trust the generated CA too.
func TestTLSAutoServiceLinkEnv(t *testing.T) {
	c := newTestAllCmd()
	c.TLSAuto = core.TLSAuto{Auto: true, CADir: "/ca"}
	got := map[string]string{}
	for _, e := range c.serviceLinkEnv() {
		got[e.Key] = e.Value
	}
	if v, want := got[core.CACertPathEnv], filepath.Join("/ca", "ca.pem"); v != want {
		t.Errorf("%s = %q, want %q", core.CACertPathEnv, v, want)
	}
	if v, want := got["SAKURA_ENDPOINTS_KMS"], "https://127.0.0.1:18000/kms"; v != want {
		t.Errorf("SAKURA_ENDPOINTS_KMS = %q, want %q", v, want)
	}
}
