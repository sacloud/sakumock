package monitoringsuite

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local monitoring-suite server.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18084" env:"MONITORINGSUITE_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"MONITORINGSUITE_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"MONITORINGSUITE_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"MONITORINGSUITE_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"MONITORINGSUITE_DEBUG" default:"false"`

	// Telemetry data plane (ingest only) options. When enabled, a second
	// listener accepts Prometheus remote-write (metrics) and OTLP/HTTP (logs,
	// traces); received payloads are acknowledged and, for debugging, optionally
	// logged or written as JSON. There is no query side. See dataplane.go.
	EnableDataPlane  bool   `help:"Serve the telemetry data plane: Prometheus remote-write (metrics) and OTLP/HTTP (logs, traces). Ingest only — payloads are acknowledged, not queryable." env:"MONITORINGSUITE_ENABLE_DATA_PLANE" default:"false"`
	DataPlaneAddr    string `help:"Listen address for the telemetry data plane (control-plane port + 10000)" env:"MONITORINGSUITE_DATA_PLANE_ADDR" default:"127.0.0.1:28084"`
	DataPlaneDumpDir string `help:"Write each received telemetry payload as JSON to this directory (debugging); empty disables file dumps. Combine with --debug to also log payloads." env:"MONITORINGSUITE_DATA_PLANE_DUMP_DIR"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own.
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger

	// tls is the common certificate/key pair the data plane serves HTTPS with
	// when both files are set; empty means plain HTTP. The standalone binary sets
	// it from --tls-cert/--tls-key (cli.go) and the unified binary injects it via
	// NewServer (ServerOptions.TLS).
	tls core.TLSFiles
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock. The monitoring-suite SDK reads
// the SAKURA_ENDPOINTS_MONITORING_SUITE override (service key "monitoring_suite").
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_MONITORING_SUITE", Value: "http://" + c.Addr},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "monitoringsuite" }

// ListenAddr returns the configured listen address.
func (c Config) ListenAddr() string { return c.Addr }

// NewServer builds the mock server, adapting NewHandler to core.ServiceConfig.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
	c.tls = opts.TLS
	return NewHandler(c)
}

// Compile-time checks that the service satisfies the core interfaces.
var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is a local monitoring-suite-compatible test server.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	rateLimiter *core.RateLimiter
	logger      *slog.Logger
	// dataPlane is the telemetry ingest listener when --enable-data-plane is set;
	// nil otherwise (its methods are nil-safe).
	dataPlane *dataPlane
}

// NewHandler creates a Server as an http.Handler without starting a listener.
func NewHandler(cfg Config) (*Server, error) {
	s := &Server{
		store:   NewStore(),
		latency: cfg.Latency,
		rateLimiter: core.NewRateLimiter(
			cfg.RateLimit,
			core.WithRateLimitWindow(cfg.RateLimitWindow),
			core.WithRateLimitErrorWriter(func(w http.ResponseWriter, status int, message string) {
				writeError(w, status, message)
			}),
		),
	}
	if cfg.idGen != nil {
		s.store.ids = cfg.idGen
	}
	base := cfg.logger
	if base == nil {
		base = slog.Default()
	}
	s.logger = base.With("service", cfg.Name())
	s.mux = s.buildMux()
	if cfg.EnableDataPlane {
		dp, err := startDataPlane(cfg, s.logger)
		if err != nil {
			s.store.Close()
			return nil, err
		}
		s.dataPlane = dp
	}
	return s, nil
}

// NewTestServer creates and starts a new local monitoring-suite test server.
func NewTestServer(cfg Config) *Server {
	s, err := NewHandler(cfg)
	if err != nil {
		panic(err)
	}
	s.httpServer = httptest.NewServer(s)
	return s
}

// TestURL returns the base URL of the test server.
func (s *Server) TestURL() string {
	return s.httpServer.URL
}

// DataPlaneAddr returns the telemetry data plane's listen address, or "" when
// the data plane is disabled.
func (s *Server) DataPlaneAddr() string {
	return s.dataPlane.Addr()
}

// Close shuts down the test server (if running) and closes the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.dataPlane.Close()
	s.store.Close()
}
