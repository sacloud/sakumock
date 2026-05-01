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

Sequential from 18080. Next available: 18084.

### Go version policy

- Support one version behind the latest stable Go release (e.g., if Go 1.26 is the latest, use Go 1.25)
- Do not depend on features available only in the latest Go version

### OpenAPI specs

- Each service has an `openapi/` directory containing the API spec copied from the SDK module
- Run `make openapi` in each service directory to fetch the spec from the Go module cache
- When upgrading an SDK dependency, always run `make openapi` to update the spec
- Handler implementations must conform to the OpenAPI spec (paths, methods, request/response schemas, status codes)
- Error responses must also conform to the spec's error schema (e.g., `components/schemas/Error`); do not invent ad-hoc shapes like `{"error": "..."}`

### Code style

- Logging: `log/slog` (Info for requests, Debug for operations)
- CLI: `alecthomas/kong` for flag parsing
- Tests: use the real SAKURA Cloud SDK client against `NewTestServer`
- SDK endpoint: `SAKURA_ENDPOINTS_<SERVICE_KEY>` environment variable
