package apprundedicated

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds the AppRun Dedicated mock server's options.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18089" env:"APPRUN_DEDICATED_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"APPRUN_DEDICATED_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"APPRUN_DEDICATED_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"APPRUN_DEDICATED_RATE_LIMIT_WINDOW"`
	Fault           []string      `help:"Inject faults: CODE:RATE[:PHASE], repeatable — return HTTP status CODE (or drop the connection when CODE is 'reset') with probability RATE, before (default) or after running the handler" placeholder:"CODE:RATE[:PHASE]" env:"APPRUN_DEDICATED_FAULT"`
	Debug           bool          `help:"Enable debug mode" env:"APPRUN_DEDICATED_DEBUG" default:"false"`

	EnableDataPlane bool   `help:"Enable data plane (reverse proxy to Docker containers)" env:"APPRUN_DEDICATED_ENABLE_DATA_PLANE" default:"false"`
	DataPlaneAddr   string `help:"Data plane address (control-plane port + 10000)" env:"APPRUN_DEDICATED_DATA_PLANE_ADDR" default:"127.0.0.1:28089"`

	logger *slog.Logger
	tls    core.TLSFiles
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock.
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_APPRUN_DEDICATED", Value: "http://" + c.Addr},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "apprun-dedicated" }

// ListenAddr returns the configured listen address.
func (c Config) ListenAddr() string { return c.Addr }

// NewServer builds the mock server with the shared options.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.logger = opts.Logger
	c.tls = opts.TLS
	return NewHandler(c)
}

var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is the AppRun Dedicated mock server. It is an http.Handler, so it can be mounted
// directly or started on a local listener with NewTestServer.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
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
	docker        *DockerManager
	dp            *dataPlane
	dataPlaneAddr string
	tlsEnabled    bool
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
		store:   NewStore(),
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
	s.mux = s.buildMux()

	if cfg.EnableDataPlane {
		s.docker = NewDockerManager(logger)
		s.dataPlaneAddr = cfg.DataPlaneAddr
		s.tlsEnabled = cfg.tls.Enabled()
		dp, err := startDataPlane(cfg, s.docker, s.store, logger)
		if err != nil {
			return nil, err
		}
		s.dp = dp
	}

	return s, nil
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, rr := range s.routeTable() {
		mux.HandleFunc(rr.Route.Method+" "+rr.Route.Path, rr.Handler)
	}
	return mux
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

// DataPlaneAddr is the data plane's listen address, or "" when it is disabled.
func (s *Server) DataPlaneAddr() string {
	if s.dp == nil {
		return ""
	}
	return s.dp.Addr()
}

// Close stops the test server and releases the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.dp != nil {
		s.dp.Close()
	}
	s.store.Close()
}
