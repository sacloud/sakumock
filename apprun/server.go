package apprun

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local AppRun server.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18088" env:"APPRUN_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"APPRUN_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"APPRUN_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"APPRUN_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"APPRUN_DEBUG" default:"false"`

	EnableDataPlane bool   `help:"Enable data plane (reverse proxy to Docker containers)" env:"APPRUN_ENABLE_DATA_PLANE" default:"false"`
	DataPlaneAddr   string `help:"Data plane address (control-plane port + 10000)" env:"APPRUN_DATA_PLANE_ADDR" default:"127.0.0.1:28088"`

	idGen  *core.IDGenerator
	logger *slog.Logger
	tls    core.TLSFiles
}

// ClientEnv returns the environment variables a client sets to reach this mock.
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_APPRUN_SHARED", Value: "http://" + c.Addr},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "apprun" }

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

// Server is a local AppRun-compatible test server.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	rateLimiter *core.RateLimiter
	logger      *slog.Logger
	docker      *DockerManager
	dp          *dataPlane
}

// NewHandler creates a Server as an http.Handler without starting a listener.
func NewHandler(cfg Config) (*Server, error) {
	base := cfg.logger
	if base == nil {
		base = slog.Default()
	}
	logger := base.With("service", cfg.Name())

	dataPlaneAddr := cfg.DataPlaneAddr
	publicURLFunc := func(appID string) string {
		host, port, err := net.SplitHostPort(dataPlaneAddr)
		if err != nil {
			return fmt.Sprintf("http://%s.localhost:28088", appID)
		}
		_ = host
		return fmt.Sprintf("http://%s.localhost:%s", appID, port)
	}

	s := &Server{
		store:   NewStore(publicURLFunc),
		latency: cfg.Latency,
		logger:  logger,
		rateLimiter: core.NewRateLimiter(
			cfg.RateLimit,
			core.WithRateLimitWindow(cfg.RateLimitWindow),
			core.WithRateLimitErrorWriter(func(w http.ResponseWriter, status int, message string) {
				writeAppError(w, status, message)
			}),
		),
	}
	if cfg.idGen != nil {
		s.store.ids = cfg.idGen
	}
	s.mux = s.buildMux()

	if cfg.EnableDataPlane {
		s.docker = NewDockerManager(logger, s.store)
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

// NewTestServer creates and starts a new local AppRun test server using httptest.
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

// DataPlaneAddr returns the data plane's listen address, or "" when disabled.
func (s *Server) DataPlaneAddr() string {
	if s.dp == nil {
		return ""
	}
	return s.dp.Addr()
}

// Close shuts down the test server (if running) and releases resources.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.dp != nil {
		s.dp.Close()
	}
	s.store.Close()
}
