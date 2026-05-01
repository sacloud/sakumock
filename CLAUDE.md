# sakumock

## Module Conventions

Each service is an independent Go module under its own subdirectory. Shared building blocks (e.g., `Route` type, `PrintRoutes` formatter) live in the `core/` module at `github.com/sacloud/sakumock/core`; each service depends on it via `replace ../core` and a top-level `go.work` makes local development transparent.

### Required public API

- `Config` struct with `alecthomas/kong` tags
- `NewHandler(cfg Config) *Server` — creates `http.Handler` without listener
- `NewTestServer(cfg Config) *Server` — creates and starts `httptest.Server`
- `Server.TestURL() string` — returns base URL
- `Server.Routes() []core.Route` — returns metadata for every HTTP endpoint registered on the server (the CLI's `--routes` flag prints these via `core.PrintRoutes`)
- `Server.Close()` — shuts down server and releases resources

### File structure

- `store.go` — Store interface and domain types
- `store_memory.go` — in-memory Store implementation
- `new_store.go` — Store factory
- `handler.go` — HTTP handlers and JSON types
- `route.go` — `routeTable()` (single source of truth driving both `buildMux()` and `Routes()`) plus the public `Routes()` method, all built on the shared types in `github.com/sacloud/sakumock/core`
- `server.go` — Config, Server, NewHandler, NewTestServer
- `cmd/sakumock-<service>/` — CLI entrypoint (graceful shutdown, slog, `--routes` flag)
- `.tagpr` — per-module tagpr config (`tagPrefix = <service>/`, `versionFile = <service>/version.go`); the release workflow auto-discovers any subdir containing both `go.mod` and `.tagpr`
- `version.go` — `const Version = "..."`, kept in sync with the git tag by tagpr
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
- Error responses must conform to the spec's error schema when one is defined (e.g., `commonserviceitem`-based endpoints share the SAKURA Cloud standard `Error { is_fatal, serial, status, error_code, error_msg }`)
- When the spec does not define an error schema (typical for service-specific proprietary endpoints), pick a shape that matches the real API's behavior rather than inventing an ad-hoc one

### Code style

- Logging: `log/slog` (Info for requests, Debug for operations)
- CLI: `alecthomas/kong` for flag parsing
- Tests: use the real SAKURA Cloud SDK client against `NewTestServer`
- SDK endpoint: `SAKURA_ENDPOINTS_<SERVICE_KEY>` environment variable
