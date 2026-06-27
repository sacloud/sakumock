# sakumock

## Service Conventions

The repository is a single Go module (`github.com/sacloud/sakumock`); each service is a package under its own subdirectory. Shared building blocks live in the `core/` package at `github.com/sacloud/sakumock/core` — the `Route` type and `PrintRoutes` formatter, the CLI serve helpers (`Serve`, `NotifyContext`, `SetupLogger`, `RateLimitHint`), the common TLS support (`TLSFiles` + `ServeListener` + `WithTLSScheme`, see "TLS" below), and the `IDGenerator` for sequential numeric resource IDs (services pass their own base value). Services must not import each other; shared code goes through `core`.

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

### Mock-only endpoints (`/_sakumock/`)

Endpoints that do not exist in the real SAKURA Cloud API — helpers to observe or drive the mock (inspect accepted messages, reset state, inject an event, force a fire) — share one convention so they never collide with the real API surface and are clearly separated in listings:

- **Path**: under the `/_sakumock/` prefix (e.g. `GET /_sakumock/messages`, `POST /_sakumock/events`). The prefix is reserved and never used by a real API path.
- **Route `Kind`**: `"inspection"` (the real API endpoints use `"api"`). `core.PrintRoutes` groups these under an `Inspection:` heading after the `API:` ones, so `--routes` shows at a glance what is mock-only. This holds even for endpoints that actively drive behavior (not just passive inspection/reset) — keep them all under `inspection` rather than introducing a new kind.
- **Rate limiting**: mock-only endpoints do not consume rate-limit tokens (they are test/inspection helpers, not part of the simulated API budget).
- **Naming**: `/_sakumock/<noun>` for state (`/messages`, `/events`); append a verb sub-path for an action on a resource (`/_sakumock/alerts/{id}/fire`).

Precedent: simplenotification exposes `GET`/`DELETE /_sakumock/messages` to list and clear accepted notifications.

### Unified binary & release

- The unified binary lives at `cmd/sakumock`. Because the repository is a single module, it always compiles against the current source of every service — there are no per-service module versions to pin or bump.
- `cmd/sakumock/main.go` registers each service's `Command` as a kong subcommand and dispatches via `kong.BindTo(ctx, (*context.Context)(nil))` so each `Command.Run(ctx)` receives the signal-aware context from `core.NotifyContext`.
- `cmd/sakumock/all.go` defines `serviceConfigs`, a struct embedding every service `Config` with a kong `prefix:` (e.g. `--kms-latency`) plus a `configs() []core.ServiceConfig`. It is shared by the suite-wide subcommands (`all`, `env`) so they expose the same per-service flags and iterate the same services. **When adding a new service, add its `Config` field to `serviceConfigs` and an entry to `configs()`**; nothing else in `all.go`/`env.go` changes.
  - `AllCmd` embeds `serviceConfigs` and runs the `sakumock all` subcommand: every service in one process (each on its own port), shutting them all down if any one fails. `build()` loops over `configs()` so name, address, endpoints, and construction all come through the interface — no per-service code. `--listen-host HOST` rebinds every service to `HOST` keeping each configured port (e.g. `0.0.0.0` so a container's published ports are reachable); `bindAddr()` applies it.
  - `cmd/sakumock/env.go` adds the `sakumock env` subcommand (also embeds `serviceConfigs`): it emits the client dotenv (each service's `Config.ClientEnv()` plus `core.DummyCredentialEnv()`) to stdout, or to `--output PATH` via `core.WriteEnvFile`, **without starting any server** — so it is the single way to obtain the client env, and works for a client that reaches sakumock over the network (a container). `--host HOST` substitutes the host the client actually uses into every endpoint URL, keeping the port (`withHost()`); without it the endpoints point at each service's configured listen address. `--output` exists because the published image is shell-less (distroless), so `>` redirection is unavailable for a compose oneshot.
- `cmd/sakumock/config.go` adds `sakumock all --config PATH`, a YAML/JSON file (by extension) of per-service options. It is a kong resolver, so CLI flags override it and any flag the user omits falls back to the file then its default. Keys are nested per service: the resolver splits a flag name on the first `-` (e.g. `kms-latency` → `kms` → `latency`), so a new service needs no change here — its prefixed flags become a config group automatically.
- Release flow: tagpr maintains a single release PR for the repository using bare `vX.Y.Z` tags (`.tagpr`, `versionFile = version.go`). When merged, `release.yml` runs in the same job: tagpr creates the tag and a **draft** GitHub Release with auto-generated notes (categorized via `.github/release.yml`, `.tagpr` `release = draft`), then GoReleaser builds the single cross-platform binary, attaches it to that draft (`release.mode: keep-existing` + `use_existing_draft: true`, so the notes are left intact), and publishes it. The draft is required because GitHub immutable releases freeze a published release's assets — uploading after publish fails with 422, so assets are added while still a draft and the release is published last. Running GoReleaser in the same run avoids the `GITHUB_TOKEN` limitation where a token-pushed tag does not trigger a new workflow.
- Container image: the same GoReleaser run builds and pushes a multi-platform image (`linux/amd64`, `linux/arm64`) to `ghcr.io/sacloud/sakumock` (`:<version>` and `:latest`; GoReleaser's `{{ .Version }}` is the tag with the leading `v` stripped, so the image tag has no `v`). It uses the stable `dockers` (one per-arch image each) + `docker_manifests` (combines them into one multi-arch tag) blocks rather than the experimental `dockers_v2`, because CI pins GoReleaser to `~> v2` where the stable blocks remain supported. The `Dockerfile` is COPY-only (distroless/static base) — GoReleaser feeds the prebuilt binary per platform, so no compilation/emulation happens in the image build. Its default `CMD` is `all --listen-host 0.0.0.0`. `release.yml` sets up buildx + QEMU and logs in to ghcr with the workflow `GITHUB_TOKEN` (needs `packages: write`). Because the build is COPY-only, arm64 builds on the amd64 runner without running anything under emulation.
  - A second variant image is pushed with the `-dataplane` suffix (`:<version>-dataplane`, `:latest-dataplane`) from `Dockerfile.dataplane`. It enables **every** service's data plane by default, each bound to `0.0.0.0` so the published ports are reachable: Object Storage S3 (`OBJECT_STORAGE_ENABLE_DATA_PLANE=true`, `OBJECT_STORAGE_DATA_PLANE_ADDR=0.0.0.0:28086`, backend dir `/home/nonroot/data`, writable by the distroless nonroot uid), Monitoring Suite telemetry ingest (`MONITORINGSUITE_ENABLE_DATA_PLANE=true`, `MONITORINGSUITE_DATA_PLANE_ADDR=0.0.0.0:28084`), AppRun (`APPRUN_ENABLE_DATA_PLANE=true`, `APPRUN_DATA_PLANE_ADDR=0.0.0.0:28088`), and AppRun Dedicated (`APPRUN_DEDICATED_ENABLE_DATA_PLANE=true`, `APPRUN_DEDICATED_DATA_PLANE_ADDR=0.0.0.0:28089`). **When adding a new service with a data plane (`--enable-data-plane`), enable it here too**: set its `<SERVICE>_ENABLE_DATA_PLANE=true` and `<SERVICE>_DATA_PLANE_ADDR=0.0.0.0:<port>` in `Dockerfile.dataplane`'s `ENV`, add the port to `EXPOSE`, and bundle any external binary it needs (as versitygw and docker are). External binaries are COPYed from their official multi-arch images (versitygw from `ghcr.io/versity/versitygw`, Docker CLI from `docker:29-cli`; both pinned by digest) — buildx resolves each image for the build's target platform, so the matching arch is selected automatically and the final stage stays COPY-only (no download/checksum/emulation; the digest provides integrity). The AppRun data planes require the host Docker socket mounted (`-v /var/run/docker.sock:/var/run/docker.sock`) and `--network host` so the reverse proxy can reach the spawned containers; users who don't need AppRun data planes can disable them with `APPRUN_ENABLE_DATA_PLANE=false`/`APPRUN_DEDICATED_ENABLE_DATA_PLANE=false`. The default image deliberately omits versitygw, docker, and all data planes to stay lean.
- The released binary and `go install .../cmd/sakumock@latest` both build the single module at the tagged commit, so they always match the repository source. Historical per-service module tags (e.g. `kms/v0.1.0`) predate the consolidation; library consumers should depend on `github.com/sacloud/sakumock` itself (import paths are unchanged) and drop any require on the old per-service module paths to avoid ambiguous-import errors.
- A single binary means GoReleaser OSS suffices (the multi-binary monorepo feature is Pro-only), which is the whole reason for aggregating.

### Port allocation

Control-plane ports are sequential from 18080. Next available: 18091. (18080 simplemq, 18081 kms, 18082 secretmanager, 18083 simplenotification, 18084 monitoringsuite, 18085 eventbus, 18086 objectstorage, 18087 iam, 18088 apprun, 18089 apprundedicated, 18090 workflows.)

A data plane needs a **separate listener** (control-plane port + 10000) when the protocol or handler is fundamentally different from the control-plane HTTP API:

- **objectstorage** (18086 → 28086): the S3 data plane is an external versitygw process, not part of the sakumock binary
- **monitoringsuite** (18084 → 28084): the telemetry ingest accepts Prometheus remote-write (snappy-compressed protobuf) and OTLP/HTTP — different wire formats from the JSON control-plane API
- **apprun / apprundedicated** (18088 → 28088, 18089 → 28089): Docker reverse proxies that route by Host header to container ports — host-based routing conflicts with control-plane path routing on the same listener

The large offset keeps the data-plane band (28080+) clear of the growing control-plane band (18080+) — they only collide at ~10000 services — while staying a trivial arithmetic mapping with a shared suffix (18086 ↔ 28086). Do not use a smaller offset such as +100, which collides once 100 services exist.

A data plane that is just additional HTTP paths or an internal engine serves on the **same port** (no `DATA_PLANE_ADDR`):

- **simplemq** (18080): queue management (control) and message send/receive (data) are both HTTP JSON on different path prefixes, served by the same mux
- **workflows** (18090): the Runbook execution engine (`--enable-data-plane`) runs in-process; executions are triggered and observed through the same control-plane API

### TLS

- TLS is a single **common** option, not per-service: one certificate/key pair serves every listener (all control planes and all data planes) over HTTPS, because they all run on the same host and differ only by port. It is enabled only when **both** files are set; otherwise everything stays plain HTTP. Setting **exactly one** is a startup error (`TLSFiles.Validate`, called by each command's `Run`) rather than silently serving plain HTTP.
- `core.TLSFiles{CertFile, KeyFile}` is the shared type (with `Enabled()`/`Scheme()`). Control planes serve via `core.Serve(ctx, addr, h, tls)` (HTTPS when enabled); in-process data planes serve via `core.ServeListener(srv, ln, tls)`. Embed it in a `Command` with a kong `prefix:"tls-"`/`envprefix:"<SERVICE>_TLS_"` for `--tls-cert`/`--tls-key`.
- The TLS files reach a service's **data plane** (started inside `NewHandler` from `Config`) through an unexported `Config.tls` field — set by the standalone `cli.go` (`c.Config.tls = c.TLS`) and injected by the unified binary via `core.ServerOptions.TLS` in `NewServer` (same pattern as `idGen`/`logger`). The unified binary exposes one suite-wide `--tls-cert`/`--tls-key` (env `SAKUMOCK_TLS_*`) on `serviceConfigs`, so there are no per-service TLS flags under `sakumock all`/`env`.
- An **externally served** data plane is handed the same files rather than sakumock terminating TLS: objectstorage passes `--cert`/`--key` to the versitygw subprocess. A new external data plane should do likewise.
- `core.WithTLSScheme(vars, enabled)` upgrades `http://` endpoint values in the client env to `https://` (credentials/regions untouched), so `cli.go` startup logs and `sakumock env` output reflect a TLS listener. `ClientEnv()`/`ExtraClientEnv()` themselves stay `http://` (single source); the scheme is applied at the edges.

### Service link (cross-service forwarding)

Service link is an opt-in feature (`sakumock all --enable-service-link` / `SAKUMOCK_ENABLE_SERVICE_LINK=true`) that makes services forward requests to each other over HTTP, just as the real SAKURA Cloud platform does. It is `sakumock all`-only: standalone services cannot know each other's addresses. When disabled (the default), firings are recorded but not forwarded — the mock works in isolation.

#### Architecture

- `core.ServerOptions.ServiceEndpoints` (`map[string]string`, service name → base URL) carries the endpoint map from the unified binary to every service. `AllCmd.serviceEndpointMap()` builds it from the configured addresses and TLS scheme.
- Each service that supports forwarding reads `opts.ServiceEndpoints` in its `NewServer` and stores it in an unexported `Config.serviceEndpoints` field (same injection pattern as `idGen`/`logger`/`tls`).
- The forwarding logic lives in a `forwarder` struct inside the sending service's package (e.g. `eventbus/forwarder.go`). It uses the **official SDK client** to talk to the destination service — never raw `net/http`. This ensures the mock exercises the same wire protocol the real service would use.
- The forwarder is constructed in `NewHandler` when `serviceEndpoints` is non-empty, and wired into the data plane. `NewTestServerWithEndpoints(cfg, endpoints)` is the test helper that enables it without `ServerOptions`.

#### How forwarding works (EventBus example)

1. `dataPlane.fire()` resolves a process configuration's `Destination` and `Parameters`.
2. If a `forwarder` is present and the delivery succeeded, it calls `forwarder.forward(ctx, delivery)`.
3. `forward()` applies a fixed timeout (`serviceLinkTimeout`, 5s) via `context.WithTimeout` and dispatches by destination name.
4. `forwardToSimpleMQ()` parses the Parameters JSON (`{"queue_name": "...", "content": "..."}`), creates an SDK `MessageOp`, and calls `op.Send(ctx, content)`.
5. A non-empty error string from `forward()` is recorded in `Delivery.Error` and the firing is marked as failed.

#### Context propagation

`context.Context` flows from the HTTP handler (`r.Context()`) through `injectEvent`/`tick` → `fire` → `forwarder.forward` → SDK call. The autonomous scheduler (`dataPlane.run`) derives a cancellable context from its stop channel.

#### Adding a new destination

1. In `forwarder.go`, add the SDK client field and initialize it in `newForwarder()` using `saclient.Client.SetEnviron` with the appropriate `SAKURA_ENDPOINTS_*` key.
2. Add a `case` in `forward()` dispatching to a new `forwardTo<Service>()` method.
3. In the method, parse the Parameters JSON, create the SDK operation, and call it with the passed `ctx`.
4. Add an integration test in `forwarder_test.go`: start both services' test servers, wire the endpoint map, fire, and verify the message arrived.

#### Current destinations

| Source | Destination | Parameters | SDK used |
|--------|-------------|-----------|----------|
| EventBus | SimpleMQ | `{"queue_name": "...", "content": "..."}` | `simplemqsdk.NewMessageClient` + `NewMessageOp.Send` |
| EventBus | SimpleNotification | `{"group_id": "...", "message": "..."}` | `simplenotificationsdk.NewClient` + `NewGroupOp.SendMessage` |

### Resource ID generation

- Real SAKURA Cloud resource IDs are a single **global incremental** counter shared across all resource types (a queue, a KMS key, a server, etc. all draw from one monotonic sequence, so an ID is unique platform-wide). They are 12-digit numbers; the counter currently sits in the `11xx`–`12xx` band.
- Mocks generate control-plane resource IDs via `core.IDGenerator` starting at `core.DefaultIDBase()`, a time-derived 12-digit number of the form `9TTTTTTTTTCC` (T = Unix seconds mod 10^9, CC = 2-digit counter space). The `9xx` band stays clear of real SAKURA Cloud IDs (currently `11xx`–`12xx`), so a mock ID that leaks to the real API hits nothing (404). The time-based base means IDs differ across process restarts, making it easier to spot stale references.
- A standalone service counts independently from the same base, so across services two resource types could share a numeric ID. The unified `sakumock all` binary avoids this: it builds every service with one shared `IDGenerator` via `core.ServerOptions{IDGen: ...}` (each service's `NewServer` applies it to the in-memory store), so IDs are globally unique across services as in the real API — important because Terraform output that reused the same ID for different resource types is confusing and can hide a mis-wired reference. Standalone use and tests pass no options and each store generates its own. Data-plane identifiers (e.g. message IDs) are not resource IDs and do not use `IDGenerator`.

### UUID generation

- Use `github.com/google/uuid` for all UUID generation — never hand-roll UUIDs with `crypto/rand` + `fmt.Sprintf`.
- `uuid.NewString()` for random v4 UUIDs (most cases: SCIM tokens, SP key IDs, etc.).
- `uuid.NewV7()` when time-ordered UUIDs are needed.

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

- Logging: `log/slog`. Per-request logs (`ServeHTTP`) MUST be **Info** level — the mock's purpose is to confirm it is handling requests, so request visibility at Info is intentional. Use Debug for internal operations (store reads/writes, etc.)
- CLI: `alecthomas/kong` for flag parsing
- JSON request/response bodies: define a named struct with `json:"..."` tags for any shape whose fields are known and fixed — including shapes used only once. Reserve `map[string]any` for genuinely dynamic or undetermined structures (e.g. settings/icon passed through verbatim). Factor a shared type when the same shape appears in more than one place (e.g. a `{"data": ...}` envelope, or a key shape reused across resources) rather than repeating a map literal. Structs make field/type mistakes a compile error and document the contract.
- Tests: use the real SAKURA Cloud SDK client against `NewTestServer`
- End-to-end Terraform test: `test/terraform/` (root module) drives the real `sakumock all` binary with the `sacloud/sakura` provider through a full apply → plan(no-diff) → destroy for one resource per service. It is behind the `terraform` build tag (so normal `go test ./...` skips it) and `t.Skip`s when the `terraform` binary is absent; run it with `go test -tags terraform ./test/terraform/`. CI runs it in the `terraform-integration` job (fetches the provider from the registry). A new service's resource should be added to `test/terraform/main.tf`.
- SDK endpoint: `SAKURA_ENDPOINTS_<SERVICE_KEY>` environment variable
