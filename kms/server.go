package kms

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds the KMS mock server's options.
type Config struct {
	Addr            string            `help:"Listen address" default:"127.0.0.1:18081" env:"KMS_LOCALSERVER_ADDR"`
	Latency         time.Duration     `help:"Artificial latency added to every response" env:"KMS_LATENCY"`
	RateLimit       float64           `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"KMS_RATE_LIMIT"`
	RateLimitWindow time.Duration     `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"KMS_RATE_LIMIT_WINDOW"`
	Keys            map[string]string `name:"key" help:"Pre-create a key with a fixed ID and key material, repeatable: ID=SECRET[@N], where SECRET is 64 hex chars (raw AES-256 key) or any string without @ (hashed with SHA-256) and N is the latest version (default 1), so ciphertexts survive a restart" placeholder:"ID=SECRET[@N]" env:"KMS_KEY"`
	Fault           []string          `help:"Inject faults: CODE:RATE[:PHASE], repeatable — return HTTP status CODE (or drop the connection when CODE is 'reset') with probability RATE, before (default) or after running the handler" placeholder:"CODE:RATE[:PHASE]" env:"KMS_FAULT"`
	Debug           bool              `help:"Enable debug mode" env:"KMS_DEBUG" default:"false"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own.
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock.
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://" + c.Addr},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "kms" }

// ListenAddr returns the configured listen address.
func (c Config) ListenAddr() string { return c.Addr }

// NewServer builds the mock server with the shared options.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
	return NewHandler(c)
}

var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is the KMS mock server. It is an http.Handler, so it can be mounted
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
	presets, err := parsePresetKeys(cfg.Keys)
	if err != nil {
		return nil, err
	}
	for _, p := range presets {
		if err := s.store.Preset(p.id, p.material, p.version); err != nil {
			return nil, fmt.Errorf("--key: %w", err)
		}
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

// Close stops the test server and releases the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.store.Close()
}
