package eventbus

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local EventBus server.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18085" env:"EVENTBUS_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"EVENTBUS_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit on API endpoints (events per --rate-limit-window, 0 disables)" default:"0" env:"EVENTBUS_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"EVENTBUS_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"EVENTBUS_DEBUG" default:"false"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own.
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger
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

// NewServer builds the mock server, adapting NewHandler to core.ServiceConfig.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
	return NewHandler(c)
}

// Compile-time checks that the service satisfies the core interfaces.
var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is a local EventBus compatible test server.
//
// TODO: cross-service delivery (data plane). Schedules and triggers are stored
// but never fire. Under `sakumock all`, schedule firing (Crontab.Next /
// RecurringStep) could deliver to the simplemq / simplenotification mocks over
// HTTP via peer endpoints injected through core.ServerOptions, and a
// monitoringsuite simulation endpoint could emit alert events for trigger
// matching. See "TODO: cross-service delivery" in README.md for the plan.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	rateLimiter *core.RateLimiter
	logger      *slog.Logger
}

// NewHandler creates a Server as an http.Handler without starting a listener.
func NewHandler(cfg Config) (*Server, error) {
	base := cfg.logger
	if base == nil {
		base = slog.Default()
	}
	logger := base.With("service", cfg.Name())
	s := &Server{
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
	}
	if cfg.idGen != nil {
		s.store.ids = cfg.idGen
	}
	s.mux = s.buildMux()
	return s, nil
}

// NewTestServer creates and starts a new EventBus test server using httptest.
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

// Close shuts down the test server (if running) and closes the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.store.Close()
}
