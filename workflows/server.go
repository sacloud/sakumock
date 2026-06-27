package workflows

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18090" env:"WORKFLOWS_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"WORKFLOWS_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"WORKFLOWS_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"WORKFLOWS_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"WORKFLOWS_DEBUG" default:"false"`

	idGen  *core.IDGenerator
	logger *slog.Logger
}

func (c Config) ClientEnv() []core.EnvVar {
	return []core.EnvVar{
		{Key: "SAKURA_ENDPOINTS_WORKFLOWS", Value: "http://" + c.Addr},
	}
}

func (Config) Name() string         { return "workflows" }
func (c Config) ListenAddr() string { return c.Addr }

func (c Config) NewServer(opts core.ServerOptions) (core.Server, error) {
	c.idGen = opts.IDGen
	c.logger = opts.Logger
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
	logger      *slog.Logger
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
	}
	if cfg.idGen != nil {
		s.store.ids = cfg.idGen
	}
	s.mux = s.buildMux()
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

func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.store.Close()
}
