package eventbus

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds the EventBus mock server's options.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18085" env:"EVENTBUS_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"EVENTBUS_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit on API endpoints (events per --rate-limit-window, 0 disables)" default:"0" env:"EVENTBUS_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"EVENTBUS_RATE_LIMIT_WINDOW"`
	Fault           []string      `help:"Inject faults: CODE:RATE[:PHASE], repeatable — return HTTP status CODE (or drop the connection when CODE is 'reset') with probability RATE, before (default) or after running the handler" placeholder:"CODE:RATE[:PHASE]" env:"EVENTBUS_FAULT"`
	EnableDataPlane bool          `help:"Run the autonomous scheduler that fires schedules on the wall clock and delivers fired jobs. The /_sakumock firing endpoints work regardless." env:"EVENTBUS_ENABLE_DATA_PLANE" default:"false"`
	Debug           bool          `help:"Enable debug mode" env:"EVENTBUS_DEBUG" default:"false"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own.
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger

	// serviceLinkEnv is the aggregated client env vars for cross-service
	// forwarding. Injected by the unified binary when service linking is enabled.
	serviceLinkEnv []core.EnvVar
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock.
//
// The URL keeps a trailing slash: the eventbus SDK matches the list-API path
// with url.JoinPath, which drops the leading slash when the endpoint URL has
// an empty path, so without the slash the SDK never injects the Provider.Class
// filter query and List would return items of every class.
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_EVENTBUS", Value: "http://" + c.Addr + "/"},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "eventbus" }

// ListenAddr returns the configured listen address.
func (c Config) ListenAddr() string { return c.Addr }

// NewServer builds the mock server with the shared options.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
	c.serviceLinkEnv = opts.ServiceLinkEnv
	return NewHandler(c)
}

var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is a local EventBus compatible test server.
//
// The data plane (see dataplane.go) fires schedules on the wall clock and
// triggers on events injected via /_sakumock/events, recording each firing as a
// Delivery. Forwarding a fired job to its Destination service (simplemq /
// simplenotification) over HTTP is a separate layer (see forwarder.go), active
// only when service linking is enabled.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	dataPlane   *dataPlane
	latency     time.Duration
	rateLimiter *core.RateLimiter
	fault       *core.FaultInjector
	// validator rejects request bodies violating the spec-derived constraints
	// in the generated bodySchemas table (validate_gen.go).
	validator *core.BodyValidator
	// respValidator checks handler responses against the generated
	// responseSchemas table. Violations never alter the response: they are
	// logged at Warn and inspectable via GET /_sakumock/spec-violations.
	respValidator *core.ResponseValidator
	logger        *slog.Logger
}

// NewHandler builds the mock server as an http.Handler, without a listener.
func NewHandler(cfg Config) (*Server, error) {
	base := cfg.logger
	if base == nil {
		base = slog.Default()
	}
	logger := base.With("service", cfg.Name())
	fault, err := core.ParseFaultSpecs(cfg.Fault, core.WithFaultErrorWriter(writeError))
	if err != nil {
		return nil, err
	}
	s := &Server{
		fault:   fault,
		store:   NewStore(logger),
		latency: cfg.Latency,
		logger:  logger,
		rateLimiter: core.NewRateLimiter(
			cfg.RateLimit,
			core.WithRateLimitWindow(cfg.RateLimitWindow),
			core.WithRateLimitErrorWriter(func(w http.ResponseWriter, status int, message string) {
				writeError(w, status, message)
			}),
		),
		validator:     core.NewBodyValidator(bodySchemas, writeError, core.WithNonEmpty(bodyNonEmptyFields)),
		respValidator: core.NewResponseValidator(responseSchemas, logger),
	}
	if cfg.idGen != nil {
		s.store.ids = cfg.idGen
	}
	s.dataPlane = newDataPlane(s.store, logger, nil)
	if len(cfg.serviceLinkEnv) > 0 {
		s.dataPlane.forwarder = newForwarder(cfg.serviceLinkEnv, logger)
	}
	if cfg.EnableDataPlane {
		s.dataPlane.start()
	}
	s.mux = s.buildMux()
	return s, nil
}

// NewTestServer builds the mock server and starts it on a local httptest
// listener. It panics if the server cannot be built.
func NewTestServer(cfg Config) *Server {
	s, err := NewHandler(cfg)
	if err != nil {
		panic(err)
	}
	s.httpServer = httptest.NewServer(s)
	return s
}

// NewTestServerWithServiceLink creates and starts a new EventBus test server
// with service linking enabled. The env vars are passed to the forwarder's
// SDK clients via saclient.Client.SetEnviron.
func NewTestServerWithServiceLink(cfg Config, env []core.EnvVar) *Server {
	cfg.serviceLinkEnv = env
	return NewTestServer(cfg)
}

// TestURL is the base URL of a server started by NewTestServer, or "" when the
// server was built with NewHandler.
func (s *Server) TestURL() string {
	if s.httpServer == nil {
		return ""
	}
	return s.httpServer.URL
}

// SpecViolations returns the OpenAPI spec violations recorded so far by the
// response validator (also served at GET /_sakumock/spec-violations).
func (s *Server) SpecViolations() []core.SpecViolation {
	return s.respValidator.Violations()
}

// Secret returns the secret set on the process configuration with the given
// ID via the set-secret endpoint. The API itself is write-only for secrets, so
// this accessor lets tests assert what an application configured.
func (s *Server) Secret(id string) (json.RawMessage, bool) {
	it, ok := s.store.GetItem(id)
	if !ok || len(it.Secret) == 0 {
		return nil, false
	}
	return it.Secret, true
}

// Close stops the test server and releases the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.dataPlane != nil {
		s.dataPlane.close()
	}
	s.store.Close()
}

// Deliveries returns the firings the data plane has recorded so far, oldest
// first. Like Secret, it lets tests assert what the mock would deliver without
// a live destination service.
func (s *Server) Deliveries() []Delivery {
	return s.dataPlane.recordedDeliveries()
}
