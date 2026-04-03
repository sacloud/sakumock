package secretmanager

import (
	"net/http"
	"net/http/httptest"
	"time"
)

// Config holds configuration for the local SecretManager server.
type Config struct {
	Addr    string        `help:"Listen address" default:"127.0.0.1:18082" env:"SECRETMANAGER_LOCALSERVER_ADDR"`
	Latency time.Duration `help:"Artificial latency added to every response" env:"SECRETMANAGER_LATENCY"`
	Debug   bool          `help:"Enable debug mode" env:"SECRETMANAGER_DEBUG" default:"false"`
}

// Server is a local SecretManager-compatible test server.
type Server struct {
	httpServer *httptest.Server
	mux        *http.ServeMux
	store      Store
	latency    time.Duration
}

// NewHandler creates a Server as an http.Handler without starting a listener.
func NewHandler(cfg Config) *Server {
	s := &Server{
		store:   NewStore(),
		latency: cfg.Latency,
	}
	s.mux = s.buildMux()
	return s
}

// NewTestServer creates and starts a new local SecretManager test server using httptest.
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
