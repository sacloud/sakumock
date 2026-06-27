package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/apprun"
	"github.com/sacloud/sakumock/apprundedicated"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/eventbus"
	"github.com/sacloud/sakumock/iam"
	"github.com/sacloud/sakumock/kms"
	"github.com/sacloud/sakumock/monitoringsuite"
	"github.com/sacloud/sakumock/objectstorage"
	"github.com/sacloud/sakumock/secretmanager"
	"github.com/sacloud/sakumock/simplemq"
	"github.com/sacloud/sakumock/simplenotification"
	"github.com/sacloud/sakumock/workflows"
)

// serviceConfigs embeds every service's Config with its CLI prefix. It is
// shared by the commands that operate on the whole suite (`all`, `env`), so
// they expose the same per-service flags and iterate the same set of services.
//
// Registering a new service here — as a field below and an entry in configs() —
// is all that is needed to add it to those commands; name, address, endpoint
// vars, and construction all come through the core.ServiceConfig interface.
type serviceConfigs struct {
	Simplemq           simplemq.Config           `embed:"" prefix:"simplemq-"`
	Kms                kms.Config                `embed:"" prefix:"kms-"`
	Secretmanager      secretmanager.Config      `embed:"" prefix:"secretmanager-"`
	Simplenotification simplenotification.Config `embed:"" prefix:"simplenotification-"`
	Monitoringsuite    monitoringsuite.Config    `embed:"" prefix:"monitoringsuite-"`
	Eventbus           eventbus.Config           `embed:"" prefix:"eventbus-"`
	Objectstorage      objectstorage.Config      `embed:"" prefix:"objectstorage-"`
	Iam                iam.Config                `embed:"" prefix:"iam-"`
	Apprun             apprun.Config             `embed:"" prefix:"apprun-"`
	ApprunDedicated    apprundedicated.Config    `embed:"" prefix:"apprun-dedicated-"`
	Workflows          workflows.Config          `embed:"" prefix:"workflows-"`

	// TLS is one common certificate/key pair applied to every service's listeners
	// (control plane and data plane). When both files are set, all listeners serve
	// HTTPS; otherwise plain HTTP. It is a single suite-wide option, not per
	// service, because every listener runs on the same host (only the port differs).
	TLS core.TLSFiles `embed:"" prefix:"tls-" envprefix:"SAKUMOCK_TLS_"`
}

// configs lists every service in start order.
func (c *serviceConfigs) configs() []core.ServiceConfig {
	return []core.ServiceConfig{c.Simplemq, c.Kms, c.Secretmanager, c.Simplenotification, c.Monitoringsuite, c.Eventbus, c.Objectstorage, c.Iam, c.Apprun, c.ApprunDedicated, c.Workflows}
}

// AllCmd runs every mock service together in a single process, each on its own
// port. Per-service flags remain available with a service prefix (e.g.
// --simplemq-addr, --kms-latency), so the defaults match the standalone
// subcommands.
type AllCmd struct {
	serviceConfigs

	Config     configFileFlag `name:"config" placeholder:"PATH" help:"Load service options from a YAML or JSON file, nested per service (e.g. 'kms: {latency: 5s}'); CLI flags override it"`
	Debug      bool           `help:"Enable debug logging for all services"`
	ListenHost string         `name:"listen-host" placeholder:"HOST" help:"Bind every service to this host instead of each service's configured address (e.g. 0.0.0.0 to accept connections from outside a container). The port is kept."`
}

// serviceInstance pairs a service's config with its running server.
type serviceInstance struct {
	cfg    core.ServiceConfig
	server core.Server
}

// bindAddr returns the address a service should listen on: its configured
// address, or — when --listen-host is set — that host with the configured port.
func (c *AllCmd) bindAddr(listenAddr string) string {
	if c.ListenHost == "" {
		return listenAddr
	}
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return listenAddr
	}
	return net.JoinHostPort(c.ListenHost, port)
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
		IDGen:  core.NewIDGenerator(core.DefaultIDBase()),
		Logger: slog.Default(),
		TLS:    c.TLS,
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

func (c *AllCmd) debug() bool {
	return c.Debug || c.Simplemq.Debug || c.Kms.Debug || c.Secretmanager.Debug || c.Simplenotification.Debug || c.Monitoringsuite.Debug || c.Eventbus.Debug || c.Objectstorage.Debug || c.Iam.Debug || c.Apprun.Debug || c.ApprunDedicated.Debug || c.Workflows.Debug
}

// Run starts every mock service and serves until ctx is canceled. If one
// service fails (e.g. its port is in use), the rest are shut down too.
func (c *AllCmd) Run(ctx context.Context) error {
	if err := c.TLS.Validate(); err != nil {
		return err
	}
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

	slog.Info("sakumock all starting", "version", sakumock.Version)
	for _, i := range instances {
		slog.Info("starting service", "service", i.cfg.Name(), "addr", c.bindAddr(i.cfg.ListenAddr()), "scheme", c.TLS.Scheme())
	}
	slog.Info("run `sakumock env` to emit a dotenv file (endpoints + dummy credentials) for your SDK / Terraform client")

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
			if err := core.Serve(ctx, c.bindAddr(inst.cfg.ListenAddr()), inst.server, c.TLS); err != nil {
				errs[idx] = fmt.Errorf("%s: %w", inst.cfg.Name(), err)
			}
		}(idx, inst)
	}
	wg.Wait()

	return errors.Join(errs...)
}
