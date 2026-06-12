package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/kms"
	"github.com/sacloud/sakumock/monitoringsuite"
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
	Monitoringsuite    monitoringsuite.Config    `embed:"" prefix:"monitoringsuite-"`

	Config       configFileFlag `name:"config" placeholder:"PATH" help:"Load service options from a YAML or JSON file, nested per service (e.g. 'kms: {latency: 5s}'); CLI flags override it"`
	Debug        bool           `help:"Enable debug logging for all services"`
	WriteEnvFile string         `name:"write-env-file" type:"path" placeholder:"PATH" help:"Write client environment variables (service endpoints + dummy credentials) to this dotenv file for your SDK / Terraform client to load"`
}

// serviceInstance pairs a service's config with its running server.
type serviceInstance struct {
	cfg    core.ServiceConfig
	server core.Server
}

// configs lists every service in start order. Registering a new service here —
// as a field above and an entry below — is all that is needed to add it to
// `sakumock all`; name, address, endpoint vars, and construction all come
// through the core.ServiceConfig interface.
func (c *AllCmd) configs() []core.ServiceConfig {
	return []core.ServiceConfig{c.Simplemq, c.Kms, c.Secretmanager, c.Simplenotification, c.Monitoringsuite}
}

// build constructs every service's server. On error it closes the servers it
// already created so no store is leaked.
func (c *AllCmd) build() ([]serviceInstance, error) {
	// Share one ID generator across all services so resource IDs are globally
	// unique like the real API, instead of each store counting from the same
	// base and minting the same ID for different resource types — which is
	// confusing when reading Terraform output. Injected through the interface,
	// so adding a service needs no change here.
	// Inject the configured default logger so each service tags its log lines
	// with its own name (service=<name>), making the interleaved output of all
	// services in one process attributable to the service that emitted it.
	opts := core.ServerOptions{
		IDGen:  core.NewIDGenerator(core.DefaultIDBase),
		Logger: slog.Default(),
	}

	var instances []serviceInstance
	for _, cfg := range c.configs() {
		srv, err := cfg.NewServer(opts)
		if err != nil {
			for _, i := range instances {
				i.server.Close()
			}
			return nil, fmt.Errorf("%s: %w", cfg.Name(), err)
		}
		instances = append(instances, serviceInstance{cfg: cfg, server: srv})
	}
	return instances, nil
}

// clientEnvVars assembles every service's endpoint overrides plus the shared
// dummy credentials, in a stable order.
func clientEnvVars(instances []serviceInstance) []core.EnvVar {
	var vars []core.EnvVar
	for _, i := range instances {
		vars = append(vars, i.cfg.ClientEnv()...)
	}
	return append(vars, core.DummyCredentialEnv()...)
}

func (c *AllCmd) debug() bool {
	return c.Debug || c.Simplemq.Debug || c.Kms.Debug || c.Secretmanager.Debug || c.Simplenotification.Debug || c.Monitoringsuite.Debug
}

// Run starts every mock service and serves until ctx is canceled. If one
// service fails (e.g. its port is in use), the rest are shut down too.
func (c *AllCmd) Run(ctx context.Context) error {
	core.SetupLogger(c.debug())

	instances, err := c.build()
	if err != nil {
		return err
	}
	defer func() {
		for _, i := range instances {
			i.server.Close()
		}
	}()

	if c.WriteEnvFile != "" {
		if err := core.WriteEnvFile(c.WriteEnvFile, clientEnvVars(instances)); err != nil {
			return fmt.Errorf("write env file: %w", err)
		}
	}

	slog.Info("sakumock all starting", "version", sakumock.Version)
	for _, i := range instances {
		slog.Info("starting service", "service", i.cfg.Name(), "addr", i.cfg.ListenAddr())
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
	errs := make([]error, len(instances))
	for idx, inst := range instances {
		wg.Add(1)
		go func(idx int, inst serviceInstance) {
			defer wg.Done()
			// If any service stops (clean shutdown or bind error), cancel the
			// shared context so the others stop as well.
			defer cancel()
			if err := core.Serve(ctx, inst.cfg.ListenAddr(), inst.server); err != nil {
				errs[idx] = fmt.Errorf("%s: %w", inst.cfg.Name(), err)
			}
		}(idx, inst)
	}
	wg.Wait()

	return errors.Join(errs...)
}
