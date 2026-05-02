package kms

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local KMS server.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18081" env:"KMS_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"KMS_LATENCY"`
	RateLimit       float64       `help:"HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"KMS_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"KMS_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"KMS_DEBUG" default:"false"`
}

// Server is a local KMS-compatible test server.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	rateLimiter *core.RateLimiter
}

// NewHandler creates a Server as an http.Handler without starting a listener.
func NewHandler(cfg Config) *Server {
	s := &Server{
		store:   NewStore(),
		latency: cfg.Latency,
		rateLimiter: core.NewRateLimiter(
			cfg.RateLimit,
			core.WithRateLimitWindow(cfg.RateLimitWindow),
			core.WithRateLimitErrorWriter(func(w http.ResponseWriter, status int, message string) {
				writeError(w, status, message)
			}),
		),
	}
	s.mux = s.buildMux()
	return s
}

// NewTestServer creates and starts a new local KMS test server using httptest.
func NewTestServer(cfg Config) *Server {
	s := NewHandler(cfg)
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
