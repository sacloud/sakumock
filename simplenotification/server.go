package simplenotification

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Config holds configuration for the local Simple Notification server.
type Config struct {
	Addr            string        `help:"Listen address" default:"127.0.0.1:18083" env:"SIMPLENOTIFICATION_LOCALSERVER_ADDR"`
	Latency         time.Duration `help:"Artificial latency added to every response" env:"SIMPLENOTIFICATION_LATENCY"`
	Exec            string        `help:"Shell command to run for each accepted message; the message body is piped to its stdin and metadata is exposed via SAKUMOCK_GROUP_ID / SAKUMOCK_MESSAGE_ID / SAKUMOCK_CREATED_AT environment variables" env:"SIMPLENOTIFICATION_EXEC"`
	RateLimit       float64       `help:"HTTP rate limit on API endpoints (events per --rate-limit-window, 0 disables)" default:"0" env:"SIMPLENOTIFICATION_RATE_LIMIT"`
	RateLimitWindow time.Duration `help:"Window for --rate-limit (e.g. 1s, 1m)" default:"1s" env:"SIMPLENOTIFICATION_RATE_LIMIT_WINDOW"`
	Debug           bool          `help:"Enable debug mode" env:"SIMPLENOTIFICATION_DEBUG" default:"false"`
}

// Server is a local Simple Notification compatible test server.
type Server struct {
	httpServer  *httptest.Server
	mux         *http.ServeMux
	store       *MemoryStore
	latency     time.Duration
	exec        string
	rateLimiter *core.RateLimiter
}

// NewHandler creates a Server as an http.Handler without starting a listener.
func NewHandler(cfg Config) *Server {
	s := &Server{
		store:   NewStore(),
		latency: cfg.Latency,
		exec:    cfg.Exec,
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

// NewTestServer creates and starts a new Simple Notification test server using httptest.
func NewTestServer(cfg Config) *Server {
	s := NewHandler(cfg)
	s.httpServer = httptest.NewServer(s)
	return s
}

// TestURL returns the base URL of the test server.
func (s *Server) TestURL() string {
	return s.httpServer.URL
}

// Messages returns all notification messages accepted by the server in send order.
// Useful for asserting in tests that an application has sent the expected notifications.
func (s *Server) Messages() []MessageRecord {
	return s.store.List()
}

// Reset clears all notification messages accepted by the server.
func (s *Server) Reset() {
	s.store.Reset()
}

// Close shuts down the test server (if running) and closes the store.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.store.Close()
}
