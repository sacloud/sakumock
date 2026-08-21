package addon

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds the Add-on mock server's options.
type Config struct {
	Addr              string        `help:"Listen address" default:"127.0.0.1:18093" env:"ADDON_LOCALSERVER_ADDR"`
	ProvisioningDelay time.Duration `help:"How long a created resource stays in the \"Running\" deployment state before list/get can see it (0 completes immediately)" env:"ADDON_PROVISIONING_DELAY"`
	Latency           time.Duration `help:"Artificial latency added to every response" env:"ADDON_LATENCY"`
	RateLimit         float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"ADDON_RATE_LIMIT"`
	RateLimitWindow   time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"ADDON_RATE_LIMIT_WINDOW"`
	Fault             []string      `help:"Inject faults: CODE:RATE[:PHASE], repeatable — return HTTP status CODE (or drop the connection when CODE is 'reset') with probability RATE, before (default) or after running the handler" placeholder:"CODE:RATE[:PHASE]" env:"ADDON_FAULT"`
	Debug             bool          `help:"Enable debug mode" env:"ADDON_DEBUG" default:"false"`

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock.
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_ADDON", Value: "http://" + c.Addr},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "addon" }

// ListenAddr returns the configured listen address.
func (c Config) ListenAddr() string { return c.Addr }

// NewServer builds the mock server with the shared options. Add-on resources
// are identified by generated names rather than SAKURA Cloud resource IDs, so
// opts.IDGen is not used.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.logger = opts.Logger
	return NewHandler(c)
}

var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is the Add-on mock server. It is an http.Handler, so it can be mounted
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
		store:   NewStore(logger, cfg.ProvisioningDelay),
		latency: cfg.Latency,
		logger:  logger,
		rateLimiter: core.NewRateLimiter(
			cfg.RateLimit,
			core.WithRateLimitWindow(cfg.RateLimitWindow),
			core.WithRateLimitErrorWriter(func(w http.ResponseWriter, status int, message string) {
				writeError(w, status, message)
			}),
		),
		validator:     core.NewBodyValidator(bodySchemas, writeError),
		respValidator: core.NewResponseValidator(responseSchemas, logger),
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
