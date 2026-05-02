package simplemq

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local SimpleMQ server.
type Config struct {
	APIKey            string        `help:"API key for authentication (if empty, any key is accepted)" env:"SIMPLEMQ_API_KEY"`
	Addr              string        `help:"Listen address" default:"127.0.0.1:18080" env:"SIMPLEMQ_LOCALSERVER_ADDR"`
	VisibilityTimeout time.Duration `help:"Visibility timeout" default:"30s" env:"SIMPLEMQ_VISIBILITY_TIMEOUT"`
	MessageExpire     time.Duration `help:"Message expire time" default:"96h" env:"SIMPLEMQ_MESSAGE_EXPIRE"`
	Database          string        `help:"SQLite database path for persistent storage" env:"SIMPLEMQ_DATABASE"`
	Latency           time.Duration `help:"Artificial latency added to every response" env:"SIMPLEMQ_LATENCY"`
	RateLimit         float64       `help:"Per-queue HTTP rate limit (events per --rate-limit-window, 0 disables)" default:"0" env:"SIMPLEMQ_RATE_LIMIT"`
	RateLimitWindow   time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"SIMPLEMQ_RATE_LIMIT_WINDOW"`
	Debug             bool          `help:"Enable debug mode" env:"SIMPLEMQ_DEBUG" default:"false"`
}

// Server is a local SimpleMQ-compatible test server.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       Store
	apiKey      string
	latency     time.Duration
	rateLimiter *core.RateLimiter
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
