# sakumock

## Module Conventions

Each service is an independent Go module under its own subdirectory. Shared building blocks live in the `core/` module at `github.com/sacloud/sakumock/core` — the `Route` type and `PrintRoutes` formatter, plus the CLI serve helpers (`Serve`, `NotifyContext`, `SetupLogger`, `RateLimitHint`). Each service depends on `core` and a top-level `go.work` makes local development transparent.

All services are also aggregated into a single `sakumock` binary built from the repository-root module (`github.com/sacloud/sakumock`, entrypoint `cmd/sakumock`). It exposes each service as a subcommand (`sakumock <service>`) with the same flags as the standalone `sakumock-<service>` binary. Only this unified binary is released as a prebuilt artifact (via GoReleaser); per-service binaries remain `go install`-able. See "Unified binary & release" below.

### Required public API

- `Config` struct with `alecthomas/kong` tags
- `Command` struct — embeds `Config`, adds a `Routes bool` flag and a `Run(ctx context.Context) error` method; reused by both the standalone `sakumock-<service>` binary and the unified `sakumock` binary
- `NewHandler(cfg Config) (*Server, error)` — creates `http.Handler` without listener (return a `nil` error when construction cannot fail, to keep the signature uniform across services)
- `NewTestServer(cfg Config) *Server` — creates and starts `httptest.Server` (panics on `NewHandler` error)
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
- `cli.go` — `Command` (embeds `Config`, adds `--routes`); its `Run` sets up logging, prints routes or starts the server via `core.Serve`, and holds the service-specific startup log lines
- `cmd/sakumock-<service>/` — standalone CLI entrypoint; a thin shim that parses flags into `Command` and calls `Command.Run` (uses `core.NotifyContext` for signal handling — no per-service signal files)
- `.tagpr` — per-module tagpr config (`tagPrefix = <service>/`, `versionFile = <service>/version.go`); the `tagpr.yml` workflow auto-discovers any subdir containing both `go.mod` and `.tagpr`
- `version.go` — `const Version = "..."`, kept in sync with the git tag by tagpr
- Makefile, README.md

### Unified binary & release

- The repository-root module `github.com/sacloud/sakumock` contains only the unified binary at `cmd/sakumock` (`main.go` + `version.go` with `const Version`). It `require`s every service module, and `go.work` includes `./` so local and CI builds compile against the current source of every service (not the published versions).
- `cmd/sakumock/main.go` registers each service's `Command` as a kong subcommand and dispatches via `kong.BindTo(ctx, (*context.Context)(nil))` so each `Command.Run(ctx)` receives the signal-aware context from `core.NotifyContext`.
- Release flow: tagpr maintains a release PR for the root module using bare `vX.Y.Z` tags (root `.tagpr`, no `tagPrefix`). When merged, `release.yml` runs in the same job — in workspace mode (`go.work`): tagpr creates the tag and the GitHub Release with auto-generated notes (categorized via `.github/release.yml`), then GoReleaser builds the single cross-platform binary and uploads it to that release with `release.mode: keep-existing` (so the notes are left intact). Running GoReleaser in the same run avoids the `GITHUB_TOKEN` limitation where a token-pushed tag does not trigger a new workflow.
- Because the release build uses `go.work`, the released binary always matches the repository source at the tagged commit. `go install .../cmd/sakumock@latest` instead builds against the service versions pinned in the root `go.mod`; bump those to keep `go install` current. The released binary is the authoritative artifact.
- A single binary means GoReleaser OSS suffices (the multi-binary monorepo feature is Pro-only), which is the whole reason for aggregating.

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
