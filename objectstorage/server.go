package objectstorage

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local Object Storage server.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18086" env:"OBJECT_STORAGE_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"OBJECT_STORAGE_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit on API endpoints (requests per --rate-limit-window, 0 disables)" default:"0" env:"OBJECT_STORAGE_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"OBJECT_STORAGE_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"OBJECT_STORAGE_DEBUG" default:"false"`

	// Data plane (S3-compatible API) options. The data plane is served by an
	// external versitygw process (looked up on PATH) backed by a local POSIX
	// directory; see dataplane.go. It is off by default and never bundled.
	EnableDataPlane    bool   `help:"Serve the S3-compatible data plane via an external versitygw process (must be installed on PATH; disabled if not found)" env:"OBJECT_STORAGE_ENABLE_DATA_PLANE" default:"false"`
	DataPlaneAddr      string `help:"Listen address for the S3 data plane (versitygw)" env:"OBJECT_STORAGE_DATA_PLANE_ADDR" default:"127.0.0.1:28086"`
	DataPlaneDir       string `help:"Backend directory for the S3 data plane; empty uses a temporary directory removed on shutdown" env:"OBJECT_STORAGE_DATA_PLANE_DIR"`
	DataPlaneAccessKey string `help:"Root access key the S3 data plane accepts" env:"OBJECT_STORAGE_DATA_PLANE_ACCESS_KEY" default:"sakumock"`
	DataPlaneSecretKey string `help:"Root secret key the S3 data plane accepts" env:"OBJECT_STORAGE_DATA_PLANE_SECRET_KEY" default:"sakumocksecret"`
	DataPlaneRegion    string `help:"Region the S3 data plane signs/validates requests for" env:"OBJECT_STORAGE_DATA_PLANE_REGION" default:"jp-north-1"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own.
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger
}

// ClientEnv returns the environment variables a client (the SAKURA Cloud SDK or
// Terraform provider) sets to reach this mock. The SDK uses the value as the API
// root URL and appends "fed/v1" or "<site>/v2" with url.JoinPath, so a bare
// "http://addr" (no trailing slash) is joined correctly.
func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_OBJECT_STORAGE", Value: "http://" + c.Addr},
	}
}

// ExtraClientEnv returns the AWS_* environment variables an aws-cli / aws-sdk
// client uses to reach the S3 data plane, or nil when the data plane is
// disabled. It satisfies core.ClientEnvExtender so `sakumock env` includes them.
// AWS_ENDPOINT_URL_S3 and AWS_DEFAULT_REGION are honored by both aws-cli and
// aws-sdk-go-v2.
func (c Config) ExtraClientEnv() []core.EnvVar {
	if !c.EnableDataPlane {
		return nil
	}
	return []core.EnvVar{
		{Key: "AWS_ENDPOINT_URL_S3", Value: "http://" + c.DataPlaneAddr},
		{Key: "AWS_ACCESS_KEY_ID", Value: c.DataPlaneAccessKey},
		{Key: "AWS_SECRET_ACCESS_KEY", Value: c.DataPlaneSecretKey},
		{Key: "AWS_DEFAULT_REGION", Value: c.DataPlaneRegion},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "objectstorage" }

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

// Server is a local Object Storage compatible test server. It implements the
// control plane only (federation and site APIs); the S3-compatible data plane
// is out of scope.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	rateLimiter *core.RateLimiter
	logger      *slog.Logger
	// dataPlane is the external S3 gateway process when --enable-data-plane is
	// set and versitygw is available; nil otherwise (its methods are nil-safe).
	dataPlane *dataPlane
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
	if cfg.EnableDataPlane {
		dp, err := startDataPlane(cfg, logger)
		if err != nil {
			return nil, err
		}
		s.dataPlane = dp
	}
	return s, nil
}

// NewTestServer creates and starts a new Object Storage test server using httptest.
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
	s.dataPlane.Close()
	s.store.Close()
}
