package main

import (
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

	if got, want := len(services), 4; got != want {
		t.Fatalf("got %d services, want %d", got, want)
	}
	if services[0].name != "simplemq" || len(services[0].envKeys) != 2 {
		t.Errorf("simplemq should expose both control- and data-plane keys, got %+v", services[0])
	}
}

func TestClientEnvVars(t *testing.T) {
	services, err := newTestAllCmd().build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer func() {
		for _, s := range services {
			s.server.Close()
		}
	}()

	rendered := strings.Join(envLines(clientEnvVars(services)), "\n")
	for _, want := range []string{
		"SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE=http://127.0.0.1:18080",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://127.0.0.1:18080",
		"SAKURA_ENDPOINTS_KMS=http://127.0.0.1:18081",
		"SAKURA_ENDPOINTS_SECRETMANAGER=http://127.0.0.1:18082",
		"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=http://127.0.0.1:18083",
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("client env missing %q\n%s", want, rendered)
		}
	}
}

func envLines(vars []core.EnvVar) []string {
	lines := make([]string, len(vars))
	for i, v := range vars {
		lines[i] = v.Key + "=" + v.Value
	}
	return lines
}
