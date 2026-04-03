# sakumock

## Module Conventions

Each service is an independent Go module under its own subdirectory.

### Required public API

- `Config` struct with `alecthomas/kong` tags
- `NewHandler(cfg Config) *Server` — creates `http.Handler` without listener
- `NewTestServer(cfg Config) *Server` — creates and starts `httptest.Server`
- `Server.TestURL() string` — returns base URL
- `Server.Close()` — shuts down server and releases resources

### File structure

- `store.go` — Store interface and domain types
- `store_memory.go` — in-memory Store implementation
- `new_store.go` — Store factory
- `handler.go` — HTTP handlers and JSON types
- `server.go` — Config, Server, NewHandler, NewTestServer
- `cmd/sakumock-<service>/` — CLI entrypoint (graceful shutdown, slog)
- Makefile, README.md

### Port allocation

Sequential from 18080. Next available: 18083.

### Code style

- Logging: `log/slog` (Info for requests, Debug for operations)
- CLI: `alecthomas/kong` for flag parsing
- Tests: use the real SAKURA Cloud SDK client against `NewTestServer`
- SDK endpoint: `SAKURA_ENDPOINTS_<SERVICE_KEY>` environment variable
