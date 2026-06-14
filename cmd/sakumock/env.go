package main

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/sacloud/sakumock/core"
)

// EnvCmd prints the client environment variables (service endpoints + dummy
// credentials) as a dotenv file to stdout, then exits without starting any
// server. It is how a client obtains the env to reach sakumock — including over
// the network from a container, where --host substitutes the host the client
// actually uses:
//
//	docker run --rm ghcr.io/sacloud/sakumock env --host localhost > sakumock.env
//
// --host substitutes the host the client actually uses (the published host, or
// the compose service name) into every endpoint URL, keeping the port.
type EnvCmd struct {
	serviceConfigs

	Host   string         `placeholder:"HOST" help:"Host the client uses to reach sakumock, substituted into every endpoint URL (the port is kept). E.g. 'localhost' for a published container port, or the compose service name. Defaults to each service's configured address."`
	Output string         `name:"output" short:"o" type:"path" placeholder:"PATH" help:"Write the dotenv to this file instead of stdout. Use it where shell redirection is unavailable, e.g. a compose oneshot on the (shell-less) container image."`
	Export bool           `help:"Prefix every line with 'export ' so the output can be sourced directly (e.g. with direnv or a plain shell)."`
	Config configFileFlag `name:"config" placeholder:"PATH" help:"Load service options from a YAML or JSON file (same format as 'all --config')"`
}

// clientEnv builds the endpoint overrides (with --host applied to the URL host)
// plus the shared dummy credentials.
func (c *EnvCmd) clientEnv() ([]core.EnvVar, error) {
	var vars []core.EnvVar
	for _, cfg := range c.configs() {
		for _, e := range cfg.ClientEnv() {
			if c.Host != "" {
				val, err := withHost(e.Value, c.Host)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", e.Key, err)
				}
				e.Value = val
			}
			vars = append(vars, e)
		}
		// Extra client env (e.g. AWS_* for a data plane) is emitted verbatim:
		// it may carry non-URL values and points at a process not affected by
		// --host (the data plane binds its own configured address).
		if ext, ok := cfg.(core.ClientEnvExtender); ok {
			vars = append(vars, ext.ExtraClientEnv()...)
		}
	}
	vars = append(vars, core.DummyCredentialEnv()...)
	// When TLS is enabled the listeners serve HTTPS, so upgrade endpoint URLs
	// (control plane and data plane) accordingly; credentials/regions are left
	// untouched by WithTLSScheme.
	return core.WithTLSScheme(vars, c.TLS.Enabled()), nil
}

// Run renders the client env to --output, or to stdout when it is unset. Logs
// go to stderr (slog default), so the stdout dotenv stays clean for redirection.
func (c *EnvCmd) Run(_ context.Context) error {
	if err := c.TLS.Validate(); err != nil {
		return err
	}
	vars, err := c.clientEnv()
	if err != nil {
		return err
	}
	if c.Output != "" {
		return core.WriteEnvFile(c.Output, vars, c.Export)
	}
	fmt.Print(core.RenderEnvFile(vars, c.Export))
	return nil
}

// withHost replaces the host of rawURL with host, preserving the port (and the
// rest of the URL). It leaves a port-less host as just host.
func withHost(rawURL, host string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if _, port, err := net.SplitHostPort(u.Host); err == nil {
		u.Host = net.JoinHostPort(host, port)
	} else {
		u.Host = host
	}
	return u.String(), nil
}
