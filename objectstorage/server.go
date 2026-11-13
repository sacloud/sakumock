package objectstorage

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds the Object Storage mock server's options.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18086" env:"OBJECT_STORAGE_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every API response" env:"OBJECT_STORAGE_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit on API endpoints (requests per --rate-limit-window, 0 disables)" default:"0" env:"OBJECT_STORAGE_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"OBJECT_STORAGE_RATE_LIMIT_WINDOW"`
	Fault           []string      `help:"Inject faults: CODE:RATE[:PHASE], repeatable — return HTTP status CODE (or drop the connection when CODE is 'reset') with probability RATE, before (default) or after running the handler" placeholder:"CODE:RATE[:PHASE]" env:"OBJECT_STORAGE_FAULT"`
	Debug           bool          `help:"Enable debug mode" env:"OBJECT_STORAGE_DEBUG" default:"false"`

	// Data plane (S3-compatible API) options. The data plane is served by an
	// external versitygw process (looked up on PATH) backed by a local POSIX
	// directory; see dataplane.go. It is off by default and never bundled.
	EnableDataPlane bool   `help:"Serve the S3-compatible data plane via an external versitygw process (requires versitygw on PATH; startup fails if it is missing)" env:"OBJECT_STORAGE_ENABLE_DATA_PLANE" default:"false"`
	DataPlaneAddr   string `help:"Listen address for the S3 data plane (versitygw)" env:"OBJECT_STORAGE_DATA_PLANE_ADDR" default:"127.0.0.1:28086"`
	DataPlaneDir    string `help:"Backend directory for the S3 data plane; empty uses <state-dir>/objectstorage-data when --state-dir is set, otherwise a temporary directory removed on shutdown" env:"OBJECT_STORAGE_DATA_PLANE_DIR"`
	DataPlaneRegion string `help:"Region the S3 data plane signs/validates requests for" env:"OBJECT_STORAGE_DATA_PLANE_REGION" default:"jp-north-1"`

	// idGen, when non-nil, is the resource ID generator injected by the unified
	// binary via NewServer; nil means the store creates its own.
	idGen *core.IDGenerator

	// logger, when non-nil, is the base logger injected by the unified binary
	// via NewServer; nil means the server falls back to slog.Default().
	logger *slog.Logger

	// tls is the common certificate/key pair. When both files are set the data
	// plane (versitygw) is launched with --cert/--key so it serves HTTPS; empty
	// means plain HTTP. Set from --tls-cert/--tls-key (cli.go) or injected by the
	// unified binary via NewServer (ServerOptions.TLS).
	tls core.TLSFiles

	// stateDir, when non-empty, is the directory the buckets, accounts and
	// permissions are persisted in (--state-dir), set by cli.go or injected by
	// the unified binary. It also hosts the data plane's backend directory
	// unless DataPlaneDir is set (see dataPlaneDir).
	stateDir string
}

// dataPlaneStateSubdir is the data plane's backend directory inside the state
// directory, next to objectstorage.json.
const dataPlaneStateSubdir = "objectstorage-data"

// dataPlaneDir returns the data plane's backend directory: DataPlaneDir when
// set, else a subdirectory of the state directory so objects survive a restart
// together with the buckets holding them, else "" (a temporary directory).
func (c Config) dataPlaneDir() string {
	if c.DataPlaneDir != "" || c.stateDir == "" {
		return c.DataPlaneDir
	}
	return filepath.Join(c.stateDir, dataPlaneStateSubdir)
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
		{Key: "AWS_ACCESS_KEY_ID", Value: dataPlaneRootID},
		{Key: "AWS_SECRET_ACCESS_KEY", Value: dataPlaneRootValue},
		{Key: "AWS_DEFAULT_REGION", Value: c.DataPlaneRegion},
	}
}

// Name returns the service's short name.
func (Config) Name() string { return "objectstorage" }
func (Config) Doc() string  { return Doc }

// ListenAddr returns the configured listen address.
func (c Config) ListenAddr() string { return c.Addr }

// NewServer builds the mock server with the shared options.
func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
	c.tls = opts.TLS
	c.stateDir = opts.StateDir
	return NewHandler(c)
}

var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

// Server is a local Object Storage compatible test server. It implements the
// control plane (federation and site APIs) itself; the S3-compatible data plane
// is not reimplemented but can be served by an external versitygw process this
// Server starts and manages when Config.EnableDataPlane is set (see dataplane.go).
type Server struct {
	httpServer *httptest.Server
	mux        *http.ServeMux
	store      *MemoryStore
	// state saves the store to --state-dir after every request that may have
	// changed it; nil when persistence is not configured.
	state       *core.StateFile
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
	// dataPlane is the external S3 gateway process when --enable-data-plane is
	// set and versitygw is available; nil otherwise (its methods are nil-safe).
	dataPlane *dataPlane
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
	s.state, err = core.OpenState(cfg.stateDir, cfg.Name(), s.store, logger)
	if err != nil {
		return nil, err
	}
	s.mux = s.buildMux()
	if cfg.EnableDataPlane {
		dp, err := startDataPlane(cfg, logger)
		if err != nil {
			s.store.Close()
			return nil, err
		}
		s.dataPlane = dp
		if err := s.syncDataPlane(); err != nil {
			dp.Close()
			s.store.Close()
			return nil, err
		}
	}
	return s, nil
}

// syncDataPlane mirrors the buckets and access keys restored from the state
// file into the freshly started data plane, through the same calls the
// handlers make on create, so both planes agree after a restart. Bucket
// directories already in a persistent backend directory are kept with their
// objects; missing ones are created empty.
func (s *Server) syncDataPlane() error {
	buckets := s.store.ListBuckets("")
	for _, b := range buckets {
		if err := s.dataPlane.createBucket(b.Name); err != nil {
			return fmt.Errorf("restore bucket %q: %w", b.Name, err)
		}
	}
	keys := s.store.accessKeys()
	for _, k := range keys {
		if err := s.dataPlane.createUser(k.ID, k.Secret); err != nil {
			return fmt.Errorf("restore access key %q: %w", k.ID, err)
		}
	}
	if len(buckets) > 0 || len(keys) > 0 {
		s.logger.Info("data plane (S3) synced with restored state", "buckets", len(buckets), "access_keys", len(keys))
	}
	return nil
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

// Close stops the test server, saves the state when --state-dir is set, stops
// the data plane, and releases the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if err := s.state.Save(); err != nil {
		s.logger.Error("failed to save state", "path", s.state.Path(), "error", err)
	}
	s.dataPlane.Close()
	s.store.Close()
}
