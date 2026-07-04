package apigw

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18091" env:"APIGW_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"APIGW_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"APIGW_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"APIGW_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"APIGW_DEBUG" default:"false"`

	EnableDataPlane bool   `help:"Enable the gateway data plane: a separate listener routing requests by Host header to the configured upstreams" env:"APIGW_ENABLE_DATA_PLANE" default:"false"`
	DataPlaneAddr   string `help:"Data plane address (control-plane port + 10000)" env:"APIGW_DATA_PLANE_ADDR" default:"127.0.0.1:28091"`

	idGen  *core.IDGenerator
	logger *slog.Logger
	tls    core.TLSFiles
}

func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_APIGW", Value: "http://" + c.Addr},
	}
}

func (Config) Name() string         { return "apigw" }
func (c Config) ListenAddr() string { return c.Addr }

func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
	c.tls = opts.TLS
	return NewHandler(c)
}

var (
	_ core.Server        = (*Server)(nil)
	_ core.ServiceConfig = Config{}
)

type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	rateLimiter *core.RateLimiter
	// validator rejects request bodies violating the spec-derived constraints
	// in the generated bodySchemas table (validate_gen.go).
	validator *core.BodyValidator
	logger    *slog.Logger
	dp        *dataPlane
}

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
		validator: core.NewBodyValidator(bodySchemas, writeError, core.WithNonEmpty(bodyNonEmptyFields)),
	}
	if cfg.idGen != nil {
		s.store.ids = cfg.idGen
	}
	s.mux = s.buildMux()

	if cfg.EnableDataPlane {
		dp, err := startDataPlane(cfg, s.store, logger)
		if err != nil {
			s.store.Close()
			return nil, err
		}
		s.dp = dp
	}
	return s, nil
}

func NewTestServer(cfg Config) *Server {
	s, err := NewHandler(cfg)
	if err != nil {
		panic(err)
	}
	s.httpServer = httptest.NewServer(s)
	return s
}

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

func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.dp != nil {
		s.dp.Close()
	}
	s.store.Close()
}
