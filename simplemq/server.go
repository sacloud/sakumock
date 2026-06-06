package simplemq

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local SimpleMQ server.
type Config struct {
	APIKey            string        `help:"API key for authentication (if empty, any key is accepted). Mutually exclusive with --strict." env:"SIMPLEMQ_API_KEY" xor:"auth"`
	Addr              string        `help:"Listen address" default:"127.0.0.1:18080" env:"SIMPLEMQ_LOCALSERVER_ADDR"`
	VisibilityTimeout time.Duration `help:"Visibility timeout" default:"30s" env:"SIMPLEMQ_VISIBILITY_TIMEOUT"`
	MessageExpire     time.Duration `help:"Message expire time" default:"96h" env:"SIMPLEMQ_MESSAGE_EXPIRE"`
	Database          string        `help:"SQLite database path for persistent storage" env:"SIMPLEMQ_DATABASE"`
	Latency           time.Duration `help:"Artificial latency added to every response" env:"SIMPLEMQ_LATENCY"`
	RateLimit         float64       `help:"Per-queue HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"SIMPLEMQ_RATE_LIMIT"`
	RateLimitWindow   time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"SIMPLEMQ_RATE_LIMIT_WINDOW"`
	Strict            bool          `help:"Strict mode: the data plane only accepts queues created via the control plane, authenticated with the queue's issued API key (from rotate-apikey). Mutually exclusive with --api-key." env:"SIMPLEMQ_STRICT" xor:"auth"`
	Debug             bool          `help:"Enable debug mode" env:"SIMPLEMQ_DEBUG" default:"false"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own. Only the
	// in-memory store honors it; the SQLite store keeps its own (it resumes IDs
	// from persisted data).
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock. SimpleMQ serves both the control
// plane (queue) and the data plane (message) on the same address, so it exposes
// both endpoint keys.
func (c Config) ClientEnv() []core.EnvVar {
	url := "http://" + c.Addr
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE", Value: url},
		{Key: "SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE", Value: url},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "simplemq" }

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

// Server is a local SimpleMQ-compatible test server.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       Store
	apiKey      string
	strict      bool
	latency     time.Duration
	rateLimiter *core.RateLimiter
	logger      *slog.Logger
}

// NewHandler creates a Server as an http.Handler without starting a listener.
// If cfg.APIKey is non-empty, the server validates that incoming requests use this key.
func NewHandler(cfg Config) (*Server, error) {
	store, err := NewStore(cfg.VisibilityTimeout, cfg.MessageExpire, cfg.Database)
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:   store,
		apiKey:  cfg.APIKey,
		strict:  cfg.Strict,
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
		if ms, ok := s.store.(*MemoryStore); ok {
			ms.ids = cfg.idGen
		}
	}
	base := cfg.logger
	if base == nil {
		base = slog.Default()
	}
	s.logger = base.With("service", cfg.Name())
	s.mux = s.buildMux()
	return s, nil
}

// NewTestServer creates and starts a new local SimpleMQ test server using httptest.
// If cfg.APIKey is non-empty, the server validates that incoming requests use this key.
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

// Close shuts down the test server (if running) and closes the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.store.Close()
}
