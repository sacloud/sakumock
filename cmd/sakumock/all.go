package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/kms"
	"github.com/sacloud/sakumock/secretmanager"
	"github.com/sacloud/sakumock/simplemq"
	"github.com/sacloud/sakumock/simplenotification"
)

// AllCmd runs every mock service together in a single process, each on its own
// port. Per-service flags remain available with a service prefix (e.g.
// --simplemq-addr, --kms-latency), so the defaults match the standalone
// subcommands.
type AllCmd struct {
	Simplemq           simplemq.Config           `embed:"" prefix:"simplemq-"`
	Kms                kms.Config                `embed:"" prefix:"kms-"`
	Secretmanager      secretmanager.Config      `embed:"" prefix:"secretmanager-"`
	Simplenotification simplenotification.Config `embed:"" prefix:"simplenotification-"`

	Debug        bool   `help:"Enable debug logging for all services"`
	WriteEnvFile string `name:"write-env-file" type:"path" placeholder:"PATH" help:"Write client environment variables (service endpoints + dummy credentials) to this dotenv file for your SDK / Terraform client to load"`
}

// mockServer is the shared shape of every service's *Server.
type mockServer interface {
	http.Handler
	Close()
}

// serviceInstance is one running mock service within the unified "all" command.
type serviceInstance struct {
	name string
	addr string
	// envKeys lists the SAKURA_ENDPOINTS_* variables a client sets to reach
	// this service. SimpleMQ exposes both the control plane (queue) and the
	// data plane (message) on the same address, so it has two keys.
	envKeys []string
	server  mockServer
}

// build constructs every service handler. On error it closes the handlers it
// already created so no store is leaked.
func (c *AllCmd) build() ([]serviceInstance, error) {
	var services []serviceInstance
	add := func(name, addr string, envKeys []string, srv mockServer, err error) error {
		if err != nil {
			for _, s := range services {
				s.server.Close()
			}
			return fmt.Errorf("%s: %w", name, err)
		}
		services = append(services, serviceInstance{name: name, addr: addr, envKeys: envKeys, server: srv})
		return nil
	}

	smq, err := simplemq.NewHandler(c.Simplemq)
	if err := add("simplemq", c.Simplemq.Addr,
		[]string{"SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE", "SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE"}, smq, err); err != nil {
		return nil, err
	}
	k, err := kms.NewHandler(c.Kms)
	if err := add("kms", c.Kms.Addr,
		[]string{"SAKURA_ENDPOINTS_KMS"}, k, err); err != nil {
		return nil, err
	}
	sm, err := secretmanager.NewHandler(c.Secretmanager)
	if err := add("secretmanager", c.Secretmanager.Addr,
		[]string{"SAKURA_ENDPOINTS_SECRETMANAGER"}, sm, err); err != nil {
		return nil, err
	}
	sn, err := simplenotification.NewHandler(c.Simplenotification)
	if err := add("simplenotification", c.Simplenotification.Addr,
		[]string{"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION"}, sn, err); err != nil {
		return nil, err
	}
	return services, nil
}

// clientEnvVars assembles the endpoint overrides for every service plus the
// shared dummy credentials, in a stable order.
func clientEnvVars(services []serviceInstance) []core.EnvVar {
	var vars []core.EnvVar
	for _, s := range services {
		for _, key := range s.envKeys {
			vars = append(vars, core.EnvVar{Key: key, Value: "http://" + s.addr})
		}
	}
	return append(vars,
		core.EnvVar{Key: "SAKURA_ACCESS_TOKEN", Value: "dummy"},
		core.EnvVar{Key: "SAKURA_ACCESS_TOKEN_SECRET", Value: "dummy"},
	)
}

func (c *AllCmd) debug() bool {
	return c.Debug || c.Simplemq.Debug || c.Kms.Debug || c.Secretmanager.Debug || c.Simplenotification.Debug
}

// Run starts every mock service and serves until ctx is canceled. If one
// service fails (e.g. its port is in use), the rest are shut down too.
func (c *AllCmd) Run(ctx context.Context) error {
	core.SetupLogger(c.debug())

	services, err := c.build()
	if err != nil {
		return err
	}
	defer func() {
		for _, s := range services {
			s.server.Close()
		}
	}()

	if c.WriteEnvFile != "" {
		if err := core.WriteEnvFile(c.WriteEnvFile, clientEnvVars(services)); err != nil {
			return fmt.Errorf("write env file: %w", err)
		}
	}

	slog.Info("sakumock all starting", "version", Version)
	for _, s := range services {
		slog.Info("starting service", "service", s.name, "addr", s.addr)
	}
	if c.WriteEnvFile != "" {
		slog.Info("wrote client env file", "path", c.WriteEnvFile,
			"load_with", "set -a; source "+c.WriteEnvFile+"; set +a")
	} else {
		slog.Info("pass --write-env-file <path> to emit a dotenv file for your SDK / Terraform client")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errs := make([]error, len(services))
	for i, s := range services {
		wg.Add(1)
		go func(i int, s serviceInstance) {
			defer wg.Done()
			// If any service stops (clean shutdown or bind error), cancel the
			// shared context so the others stop as well.
			defer cancel()
			if err := core.Serve(ctx, s.addr, s.server); err != nil {
				errs[i] = fmt.Errorf("%s: %w", s.name, err)
			}
		}(i, s)
	}
	wg.Wait()

	return errors.Join(errs...)
}
