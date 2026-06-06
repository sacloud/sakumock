package core

import (
	"log/slog"
	"net/http"
)

// Server is the common interface every service's *Server satisfies. It lets the
// unified binary treat all mock services uniformly, and each service asserts
// conformance with a compile-time `var _ core.Server = (*Server)(nil)` so a new
// service that drifts from the contract fails to build.
type Server interface {
	// ServeHTTP handles requests; every server is an http.Handler.
	http.Handler
	// Routes returns metadata for every registered HTTP endpoint (printed by
	// the CLI's --routes flag via PrintRoutes).
	Routes() []Route
	// TestURL returns the base URL when the server was started via the
	// service's NewTestServer.
	TestURL() string
	// Close shuts the server down and releases resources.
	Close()
}

// ServerOptions carries shared dependencies the unified binary injects into
// every service when it builds them. Adding a field here lets a new shared
// dependency reach all services without changing the ServiceConfig interface
// or touching per-service code.
type ServerOptions struct {
	// IDGen, when non-nil, is the resource ID generator the service should use.
	// The unified binary passes one shared generator to every service so IDs
	// are globally unique across services, as in the real API.
	IDGen *IDGenerator
	// Logger, when non-nil, is the base logger the service tags with its own
	// name (service=<name>) for every request and operation log line, so the
	// unified binary's interleaved output identifies the originating service.
	// When nil the service falls back to slog.Default().
	Logger *slog.Logger
}

// ServiceConfig is the common interface every service's Config satisfies. It
// lets the unified binary build and describe every service uniformly, without
// hard-coding per-service names, addresses, or endpoint variables. Each service
// asserts conformance with a compile-time `var _ core.ServiceConfig = Config{}`.
type ServiceConfig interface {
	// Name returns the service's short name (e.g. "simplemq").
	Name() string
	// ListenAddr returns the configured listen address (host:port).
	ListenAddr() string
	// ClientEnv returns the SAKURA_ENDPOINTS_* override(s) a client sets to
	// reach this mock, derived from the listen address.
	ClientEnv() []EnvVar
	// NewServer builds the service's mock server with the given shared options.
	NewServer(opts ServerOptions) (Server, error)
}
