package simplemq

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds the SimpleMQ mock server's options.
type Config struct {
	APIKey            string        `help:"API key for authentication (if empty, any key is accepted). Mutually exclusive with --strict." env:"SIMPLEMQ_API_KEY" xor:"auth"`
	Addr              string        `help:"Listen address" default:"127.0.0.1:18080" env:"SIMPLEMQ_LOCALSERVER_ADDR"`
	VisibilityTimeout time.Duration `help:"Visibility timeout" default:"30s" env:"SIMPLEMQ_VISIBILITY_TIMEOUT"`
	MessageExpire     time.Duration `help:"Message expire time" default:"96h" env:"SIMPLEMQ_MESSAGE_EXPIRE"`
	Database          string        `help:"SQLite database path for persistent storage" env:"SIMPLEMQ_DATABASE"`
	Latency           time.Duration `help:"Artificial latency added to every response" env:"SIMPLEMQ_LATENCY"`
	RateLimit         float64       `help:"Per-queue HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"SIMPLEMQ_RATE_LIMIT"`
	RateLimitWindow   time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"SIMPLEMQ_RATE_LIMIT_WINDOW"`
	Fault             []string      `help:"Inject faults: CODE:RATE[:PHASE], repeatable — return HTTP status CODE (or drop the connection when CODE is 'reset') with probability RATE, before (default) or after running the handler" placeholder:"CODE:RATE[:PHASE]" env:"SIMPLEMQ_FAULT"`
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

// Server is the SimpleMQ mock server. It is an http.Handler, so it can be mounted
// directly or started on a local listener with NewTestServer.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       Store
	apiKey      string
	strict      bool
	latency     time.Duration
	rateLimiter *core.RateLimiter
	// fault and cpFault inject configured faults into the data-plane and
	// control-plane routes respectively. They share the same specs but write
	// injected errors in the data-plane ({"code","message"}) and control-plane
	// (StandardError) envelopes.
	fault   *core.FaultInjector
	cpFault *core.FaultInjector
	// validator and cpValidator reject request bodies violating the
	// spec-derived constraints in the generated bodySchemas table
	// (validate_gen.go). They share the schemas but write the 400 in the
	// data-plane ({"code","message"}) and control-plane (StandardError)
	// envelopes respectively.
	validator   *core.BodyValidator
	cpValidator *core.BodyValidator
	// respValidator checks handler responses against the generated
	// responseSchemas table. One instance covers both planes: violations are
	// logged and recorded, never written to the client, so no per-envelope
	// split is needed. Inspectable via GET /_sakumock/spec-violations.
	respValidator *core.ResponseValidator
	logger        *slog.Logger
}

// NewHandler validates that incoming requests use cfg.APIKey when it is non-empty.
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
	cpFault, err := core.ParseFaultSpecs(cfg.Fault, core.WithFaultErrorWriter(func(w http.ResponseWriter, status int, message string) {
		core.WriteStandardError(w, status, "", message)
	}))
	if err != nil {
		return nil, err
	}
	store, err := NewStore(cfg.VisibilityTimeout, cfg.MessageExpire, cfg.Database, logger)
	if err != nil {
		return nil, err
	}
	s := &Server{
		fault:   fault,
		cpFault: cpFault,
		store:   store,
		apiKey:  cfg.APIKey,
		strict:  cfg.Strict,
		latency: cfg.Latency,
		logger:  logger,
		rateLimiter: core.NewRateLimiter(
			cfg.RateLimit,
			core.WithRateLimitWindow(cfg.RateLimitWindow),
			core.WithRateLimitErrorWriter(func(w http.ResponseWriter, status int, message string) {
				writeError(w, status, message)
			}),
		),
		validator: core.NewBodyValidator(bodySchemas, writeError, core.WithNonEmpty(bodyNonEmptyFields)),
		cpValidator: core.NewBodyValidator(bodySchemas, func(w http.ResponseWriter, status int, message string) {
			core.WriteStandardError(w, status, "", message)
		}, core.WithNonEmpty(bodyNonEmptyFields)),
		respValidator: core.NewResponseValidator(responseSchemas, logger),
	}
	if cfg.idGen != nil {
		if ms, ok := s.store.(*MemoryStore); ok {
			ms.ids = cfg.idGen
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
