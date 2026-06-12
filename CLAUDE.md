# sakumock

## Service Conventions

The repository is a single Go module (`github.com/sacloud/sakumock`); each service is a package under its own subdirectory. Shared building blocks live in the `core/` package at `github.com/sacloud/sakumock/core` — the `Route` type and `PrintRoutes` formatter, the CLI serve helpers (`Serve`, `NotifyContext`, `SetupLogger`, `RateLimitHint`), and the `IDGenerator` for sequential numeric resource IDs (services pass their own base value). Services must not import each other; shared code goes through `core`.

All services are also aggregated into a single `sakumock` binary (entrypoint `cmd/sakumock`). It exposes each service as a subcommand (`sakumock <service>`) with the same flags as the standalone `sakumock-<service>` binary. Only this unified binary is released as a prebuilt artifact (via GoReleaser); per-service binaries remain `go install`-able. See "Unified binary & release" below.

### Required public API

- `Config` struct with `alecthomas/kong` tags
- `Config.ClientEnv() []core.EnvVar` — the `SAKURA_ENDPOINTS_*` override(s) a client (SDK / Terraform provider) sets to reach this mock, derived from `Config.Addr`. This is the single source of truth for the service's endpoint env var name(s): both the standalone startup log and the unified binary's `env` subcommand derive from it. SimpleMQ returns two (control-plane `_QUEUE` and data-plane `_MESSAGE`, both on its single address); most services return one.
- `Config` must also satisfy `core.ServiceConfig` (`Name() string`, `ListenAddr() string`, `ClientEnv()`, `NewServer(core.ServerOptions) (core.Server, error)`) — the interface the unified binary loops over to build and describe every service without hard-coding per-service names/addresses/endpoints. `Name()` returns the service's short name (matching the subcommand, e.g. `"simplemq"`); `ListenAddr()` returns `Config.Addr`; `NewServer(opts)` applies the shared `opts` (currently `opts.IDGen`) and adapts `NewHandler`. `core.ServerOptions` carries shared dependencies the unified binary injects into every service — add a field there to reach all services without changing the interface. Asserted with `var _ core.ServiceConfig = Config{}` in `server.go`.
- `Command` struct — embeds `Config`, adds a `Routes bool` flag and a `Run(ctx context.Context) error` method; reused by both the standalone `sakumock-<service>` binary and the unified `sakumock` binary
- `NewHandler(cfg Config) (*Server, error)` — creates `http.Handler` without listener (return a `nil` error when construction cannot fail, to keep the signature uniform across services)
- `NewTestServer(cfg Config) *Server` — creates and starts `httptest.Server` (panics on `NewHandler` error)
- `Server.TestURL() string` — returns base URL
- `Server.Routes() []core.Route` — returns metadata for every HTTP endpoint registered on the server (the CLI's `--routes` flag prints these via `core.PrintRoutes`)
- `Server.Close()` — shuts down server and releases resources
- `*Server` must satisfy `core.Server` (`http.Handler` + `Routes()` + `TestURL()` + `Close()`), the interface the unified binary uses to treat every service uniformly. Each service asserts it with a compile-time `var _ core.Server = (*Server)(nil)` in `server.go`, so a new service that drifts from the contract fails to build (the convention is type-enforced, not just documented).

### File structure

- `store.go` — Store interface and domain types
- `store_memory.go` — in-memory Store implementation
- `new_store.go` — Store factory
- `handler.go` — HTTP handlers and JSON types
- `route.go` — `routeTable()` (single source of truth driving both `buildMux()` and `Routes()`) plus the public `Routes()` method, all built on the shared types in `github.com/sacloud/sakumock/core`
- `server.go` — Config, Server, NewHandler, NewTestServer
- `cli.go` — `Command` (embeds `Config`, adds `--routes`); its `Run` sets up logging, prints routes or starts the server via `core.Serve`, and holds the service-specific startup log lines
- `cmd/sakumock-<service>/` — standalone CLI entrypoint; a thin shim that parses flags into `Command` and calls `Command.Run` (uses `core.NotifyContext` for signal handling — no per-service signal files)
- Makefile, README.md

There is no per-service version: every binary reports `sakumock.Version` from the repository-root `version.go` (package `sakumock`), kept in sync with the git tag by tagpr (root `.tagpr`).

### Unified binary & release

- The unified binary lives at `cmd/sakumock`. Because the repository is a single module, it always compiles against the current source of every service — there are no per-service module versions to pin or bump.
- `cmd/sakumock/main.go` registers each service's `Command` as a kong subcommand and dispatches via `kong.BindTo(ctx, (*context.Context)(nil))` so each `Command.Run(ctx)` receives the signal-aware context from `core.NotifyContext`.
- `cmd/sakumock/all.go` defines `serviceConfigs`, a struct embedding every service `Config` with a kong `prefix:` (e.g. `--kms-latency`) plus a `configs() []core.ServiceConfig`. It is shared by the suite-wide subcommands (`all`, `env`) so they expose the same per-service flags and iterate the same services. **When adding a new service, add its `Config` field to `serviceConfigs` and an entry to `configs()`**; nothing else in `all.go`/`env.go` changes.
  - `AllCmd` embeds `serviceConfigs` and runs the `sakumock all` subcommand: every service in one process (each on its own port), shutting them all down if any one fails. `build()` loops over `configs()` so name, address, endpoints, and construction all come through the interface — no per-service code. `--listen-host HOST` rebinds every service to `HOST` keeping each configured port (e.g. `0.0.0.0` so a container's published ports are reachable); `bindAddr()` applies it.
  - `cmd/sakumock/env.go` adds the `sakumock env` subcommand (also embeds `serviceConfigs`): it emits the client dotenv (each service's `Config.ClientEnv()` plus `core.DummyCredentialEnv()`) to stdout, or to `--output PATH` via `core.WriteEnvFile`, **without starting any server** — so it is the single way to obtain the client env, and works for a client that reaches sakumock over the network (a container). `--host HOST` substitutes the host the client actually uses into every endpoint URL, keeping the port (`withHost()`); without it the endpoints point at each service's configured listen address. `--output` exists because the published image is shell-less (distroless), so `>` redirection is unavailable for a compose oneshot.
- `cmd/sakumock/config.go` adds `sakumock all --config PATH`, a YAML/JSON file (by extension) of per-service options. It is a kong resolver, so CLI flags override it and any flag the user omits falls back to the file then its default. Keys are nested per service: the resolver splits a flag name on the first `-` (e.g. `kms-latency` → `kms` → `latency`), so a new service needs no change here — its prefixed flags become a config group automatically.
- Release flow: tagpr maintains a single release PR for the repository using bare `vX.Y.Z` tags (`.tagpr`, `versionFile = version.go`). When merged, `release.yml` runs in the same job: tagpr creates the tag and a **draft** GitHub Release with auto-generated notes (categorized via `.github/release.yml`, `.tagpr` `release = draft`), then GoReleaser builds the single cross-platform binary, attaches it to that draft (`release.mode: keep-existing` + `use_existing_draft: true`, so the notes are left intact), and publishes it. The draft is required because GitHub immutable releases freeze a published release's assets — uploading after publish fails with 422, so assets are added while still a draft and the release is published last. Running GoReleaser in the same run avoids the `GITHUB_TOKEN` limitation where a token-pushed tag does not trigger a new workflow.
- Container image: the same GoReleaser run builds and pushes a multi-platform image (`linux/amd64`, `linux/arm64`) to `ghcr.io/sacloud/sakumock` (`:vX.Y.Z` and `:latest`). It uses the stable `dockers` (one per-arch image each) + `docker_manifests` (combines them into one multi-arch tag) blocks rather than the experimental `dockers_v2`, because CI pins GoReleaser to `~> v2` where the stable blocks remain supported. The `Dockerfile` is COPY-only (distroless/static base) — GoReleaser feeds the prebuilt binary per platform, so no compilation/emulation happens in the image build. Its default `CMD` is `all --listen-host 0.0.0.0`. `release.yml` sets up buildx + QEMU and logs in to ghcr with the workflow `GITHUB_TOKEN` (needs `packages: write`). Because the build is COPY-only, arm64 builds on the amd64 runner without running anything under emulation.
- The released binary and `go install .../cmd/sakumock@latest` both build the single module at the tagged commit, so they always match the repository source. Historical per-service module tags (e.g. `kms/v0.1.0`) predate the consolidation; library consumers should depend on `github.com/sacloud/sakumock` itself (import paths are unchanged) and drop any require on the old per-service module paths to avoid ambiguous-import errors.
- A single binary means GoReleaser OSS suffices (the multi-binary monorepo feature is Pro-only), which is the whole reason for aggregating.

### Port allocation

Sequential from 18080. Next available: 18086. (18080 simplemq, 18081 kms, 18082 secretmanager, 18083 simplenotification, 18084 monitoringsuite, 18085 eventbus.)

### Resource ID generation

- Real SAKURA Cloud resource IDs are a single **global incremental** counter shared across all resource types (a queue, a KMS key, a server, etc. all draw from one monotonic sequence, so an ID is unique platform-wide). They are 12-digit numbers; the counter currently sits in the `11xx`–`12xx` band.
- Mocks generate control-plane resource IDs via `core.IDGenerator` starting at `core.DefaultIDBase` (`990000000000`, the top of the 12-digit space). This keeps mock IDs realistic in length while never colliding with a real resource ID: the global counter would have to grow ~7x to reach the `9xx` band, by which point the 12-digit space would be near exhaustion and real IDs would have grown more digits. So if a test accidentally runs against the real API (e.g. an unset endpoint env var), a mock ID resolves to nothing (404) instead of a live resource.
- A standalone service counts independently from the same base, so across services two resource types could share a numeric ID. The unified `sakumock all` binary avoids this: it builds every service with one shared `IDGenerator` via `core.ServerOptions{IDGen: ...}` (each service's `NewServer` applies it to the in-memory store), so IDs are globally unique across services as in the real API — important because Terraform output that reused the same ID for different resource types is confusing and can hide a mis-wired reference. Standalone use and tests pass no options and each store generates its own. Data-plane identifiers (e.g. message IDs) are not resource IDs and do not use `IDGenerator`.

### Go version policy

- Support one version behind the latest stable Go release (e.g., if Go 1.26 is the latest, use Go 1.25)
- Do not depend on features available only in the latest Go version

### OpenAPI specs

- Each service has an `openapi/` directory containing the API spec copied from the SDK module
- Run `make openapi` in each service directory to fetch the spec from the Go module cache
- When upgrading an SDK dependency, always run `make openapi` to update the spec
- Handler implementations must conform to the OpenAPI spec (paths, methods, request/response schemas, status codes)
- Error responses must conform to the spec's error schema when one is defined. `commonserviceitem`-based endpoints share the SAKURA Cloud standard `Error { is_fatal, serial, status, error_code, error_msg }`, written via `core.WriteStandardError(w, status, code, msg)` (it derives `error_code` from the status text when code is empty). Proprietary endpoints with no spec error schema keep their own shape
- When the spec does not define an error schema (typical for service-specific proprietary endpoints), pick a shape that matches the real API's behavior rather than inventing an ad-hoc one

### Code style

- Logging: `log/slog` (Info for requests, Debug for operations)
- CLI: `alecthomas/kong` for flag parsing
- Tests: use the real SAKURA Cloud SDK client against `NewTestServer`
- End-to-end Terraform test: `test/terraform/` (root module) drives the real `sakumock all` binary with the `sacloud/sakura` provider through a full apply → plan(no-diff) → destroy for one resource per service. It is behind the `terraform` build tag (so normal `go test ./...` skips it) and `t.Skip`s when the `terraform` binary is absent; run it with `go test -tags terraform ./test/terraform/`. CI runs it in the `terraform-integration` job (fetches the provider from the registry). A new service's resource should be added to `test/terraform/main.tf`.
- SDK endpoint: `SAKURA_ENDPOINTS_<SERVICE_KEY>` environment variable
