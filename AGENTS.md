# sakumock

## Service Conventions

The repository is a single Go module (`github.com/sacloud/sakumock`); each service is a package under its own subdirectory, and shared building blocks live in `core/`. Services must not import each other; shared code goes through `core`. All services are also aggregated into a single `sakumock` binary (`cmd/sakumock`) exposing each as a subcommand with the same flags as the standalone `sakumock-<service>` binary; only the unified binary is released (GoReleaser), per-service binaries stay `go install`-able.

### Required public API

The contract is type-enforced: `server.go` asserts `var _ core.ServiceConfig = Config{}` and `var _ core.Server = (*Server)(nil)`, so a drifting service fails to build.

- `Config` — kong-tagged struct satisfying `core.ServiceConfig`: `Name()` (short name = subcommand, e.g. `"apprun-dedicated"`), `ListenAddr()` (= `Config.Addr`), `ClientEnv() []core.EnvVar` (the `SAKURA_ENDPOINTS_*` override(s) a client sets to reach this mock — the single source for the env var name, used by startup logs, `sakumock env`, and service link), `Doc()` (= `Doc`), `NewServer(opts core.ServerOptions)` (applies the shared opts, adapts `NewHandler`). Add a field to `core.ServerOptions` to inject a dependency into every service without changing the interface.
- `Command` — embeds `Config`, adds `Routes bool` / `Docs bool` and `Run(ctx) error`; shared by the standalone and unified binaries. `--routes` prints the route table, `--docs` the embedded README, then exit.
- `Doc string` — `README.md` embedded in `doc.go`.
- `NewHandler(cfg Config) (*Server, error)` — handler without listener (return `nil` error when construction cannot fail; keep the signature uniform).
- `NewTestServer(cfg Config) *Server` — starts an `httptest.Server` (panics on error).
- `*Server` satisfies `core.Server`: `http.Handler` + `Routes() []core.Route` + `TestURL()` (`""` when built by `NewHandler`, never panics) + `Close()`.

### File structure

- `store.go` (Store interface, domain types), `store_memory.go`, `new_store.go` (factory)
- `handler.go` — HTTP handlers and JSON types
- `route.go` — `routeTable()` is the single source of truth driving both `buildMux()` and `Routes()`
- `validate_gen.go` + `generate.go` — generated validation tables and their `//go:generate` directive; `validate_overrides.go` for declared spec gaps
- `server.go` — Config, Server, NewHandler, NewTestServer
- `cli.go` — `Command`; `Run` sets up logging/tracing, prints routes/docs, or serves via `core.Serve`, and holds the startup log lines
- `doc.go` — `//go:embed README.md`
- `cmd/sakumock-<service>/` — thin standalone entrypoint (`core.NotifyContext` for signals)
- Makefile, README.md, `openapi/`

There is no per-service version: everything reports `sakumock.Version` from the root `version.go`, kept in sync with the git tag by tagpr.

### Mock-only endpoints (`/_sakumock/`)

Endpoints that do not exist in the real API (inspect state, reset, inject an event, force a fire) follow one convention:

- Path under the reserved `/_sakumock/` prefix; `/_sakumock/<noun>` for state, a verb sub-path for an action (`/_sakumock/alerts/{id}/fire`).
- Route `Kind` is `"inspection"` (real API routes are `"api"`), even for endpoints that drive behavior — do not introduce another kind. `core.PrintRoutes` lists them under `Inspection:`.
- They consume no rate-limit tokens and are never fault-injected.

### Unified binary & release

- `cmd/sakumock/all.go` defines `serviceConfigs` (every `Config` embedded with a kong `prefix:`, e.g. `--kms-latency`) and `configs()`. `all`, `env`, and `docs` all iterate it through `core.ServiceConfig`, so **registering a new service there is the only change needed** for those commands. `--config PATH` (`config.go`) is a kong resolver splitting each flag on its first `-` into a per-service group, so it needs no change either; CLI flags override the file.
- `sakumock env` prints the client dotenv without starting a server (`--host` rewrites the endpoint host, `--output` writes a file because the image is shell-less).
- Release: tagpr maintains the release PR (bare `vX.Y.Z` tags, `version.go`); on merge, `release.yml` tags, creates a draft release, and GoReleaser attaches the binary and the multi-arch images (`ghcr.io/sacloud/sakumock:<version>` and `-dataplane`). The rationale (draft release, same-run GoReleaser, stable `dockers` blocks, COPY-only Dockerfiles) is commented in `.goreleaser.yaml`, `release.yml`, and the Dockerfiles.
- **A new service with a data plane (`--enable-data-plane`) must be enabled in `Dockerfile.dataplane`** (`<SERVICE>_ENABLE_DATA_PLANE=true`, `<SERVICE>_DATA_PLANE_ADDR=0.0.0.0:<port>`, `EXPOSE`), bundling any external binary as versitygw/docker are.
- Library consumers depend on `github.com/sacloud/sakumock` itself; the historical per-service module tags (`kms/v0.1.0`) predate the consolidation.

### Embedded documentation

- The binaries embed the Markdown files as they are (each service its `README.md`; the root package `README.md`, `CHANGELOG.md`, `examples/compose.yaml`, `test/terraform/*`) — the files stay the single source, nothing is generated. `sakumock docs` (`cmd/sakumock/docs.go`) derives its service topics from `serviceConfigs`, its terraform sections and provider-doc links from the `.tf` files, and its `--help` topic list from the registry.
- **Keep it generic**: whatever it shows must follow automatically from the files a new service adds; never add per-service curation or per-query tuning.
- Output is plain Markdown on stdout (no pager, no color) — it is meant to be piped into an LLM's context. Errors list the valid topics/sections so an agent self-corrects in one round trip.
- Files that are embedded are shown to agents as reference material: keep their comments accurate (e.g. no references to removed flags).

### Port allocation

- Control-plane ports are sequential from 18080, one per service; find the next free one with `grep -h 'default:"127.0.0.1:18' */server.go`.
- A data plane gets a **separate listener at control-plane port + 10000** when its protocol or routing is fundamentally different from the control-plane HTTP API (external process such as versitygw, non-JSON wire formats such as remote-write/OTLP, Host-header routing such as the AppRun proxies and the API Gateway). The +10000 offset keeps the bands apart for ~10000 services; a small offset like +100 would collide.
- A data plane that is just more HTTP paths or an in-process engine (simplemq messages, workflows runbooks) serves on the **same port**, with no `DATA_PLANE_ADDR`.

### TLS

- One **common** certificate/key pair (`core.TLSFiles`) serves every listener; it is per suite, not per service. Enabled only when both files are set; exactly one is a startup error (`TLSFiles.Validate` in each `Run`), never silent plain HTTP.
- Control planes serve via `core.Serve(ctx, addr, h, tls)`, in-process data planes via `core.ServeListener`. Embed `TLSFiles` in `Command` with `prefix:"tls-"` / `envprefix:"<SERVICE>_TLS_"`.
- Data planes started inside `NewHandler` get the files through the unexported `Config.tls` field (set by `cli.go`, injected by the unified binary via `core.ServerOptions.TLS`). An external data plane is handed the files (objectstorage passes `--cert`/`--key` to versitygw).
- `ClientEnv()` stays `http://`; `core.WithTLSScheme` upgrades the scheme at the edges (startup log, `sakumock env`).

### Fault injection

- `core.FaultInjector` (`core/fault.go`) is built in `NewHandler` with the service's own error writer (`core.ParseFaultSpecs(cfg.Fault, core.WithFaultErrorWriter(...))`; nil when unconfigured, and `Middleware` on nil is a no-op). A service with two error envelopes builds two injectors (simplemq).
- Config is per-service: `Fault []string` with `env:"<SERVICE>_FAULT"`, specs `CODE:RATE[:PHASE]` (see README).
- Wrap it **outermost** in `routeTable()` — outside auth/rate-limit/validation — so a fault can mask a would-be 401/429/400. Inspection routes stay exempt.
- Control plane only: separate-listener data planes are not fault-injected; same-port data planes are, because their routes go through `routeTable()`.

### OpenTelemetry tracing

- Configured **only through the standard OTEL env vars** (`core.TracingEnabled()`), no sakumock flags; without them every wrap is an identity, so tests are untouched.
- Each `Command.Run` calls `core.SetupTracing(ctx)` after `core.SetupLogger` and defers the shutdown. Wrap the handler at the serve call site: `core.Serve(ctx, addr, core.TraceHandler(c.Name(), h), tls)`; an in-process data plane wraps its `http.Server` handler the same way; external data planes are not instrumented.
- Log correlation (`trace_id`/`span_id`) and fault-injection span attributes are automatic via `core.RequestLogArgs` / `r.Context()`.

### Service link (cross-service forwarding)

Opt-in (`sakumock all --enable-service-link`), `sakumock all`-only: services forward requests to each other over HTTP as the real platform does. Disabled, firings are recorded but not forwarded.

- `core.ServerOptions.ServiceLinkEnv` carries every service's `ClientEnv()` (+ dummy credentials) with `--listen-host`/TLS applied, so the forwarding service never hard-codes another service's env var name. A forwarding service stores it in an unexported `Config.serviceLinkEnv` in `NewServer` and builds a `forwarder` (e.g. `eventbus/forwarder.go`) in `NewHandler` when it is non-empty; `NewTestServerWithServiceLink(cfg, env)` enables it in tests.
- The forwarder talks to the destination with the **official SDK client** configured from those env vars (`saclient.Client.SetEnviron(core.EnvStrings(env))`), never raw `net/http`, so the mock exercises the real wire protocol. Calls take the request `ctx` with a fixed timeout.
- Adding a destination: add the SDK client to `newForwarder()`, a `case` in `forward()` calling a new `forwardTo<Service>()` that parses the Parameters JSON and calls the SDK op, and an integration test in `forwarder_test.go` that starts both test servers (env from the destination's `ClientEnv()` via the `serviceLinkEnv()` helper), fires, and checks arrival.

### Resource ID generation

- Real resource IDs are one global incremental 12-digit counter (currently in the `11xx`–`12xx` band). Mocks generate control-plane IDs with `core.IDGenerator` from `core.DefaultIDBase()` (time-derived, `9…` band) so a leaked mock ID hits nothing real. Data-plane identifiers (message IDs) are not resource IDs.
- `sakumock all` shares one generator via `core.ServerOptions.IDGen` so IDs are unique across services as in the real API; standalone processes and tests generate their own.
- **User-specified IDs** (e.g. kms `--key ID=SECRET`) go through `IDGenerator.Reserve(id, service)`, not `Observe`: it records the owner so two services fixing the same ID fail at startup; return that error from `NewHandler`.

### UUID generation

Use `github.com/google/uuid` (`uuid.NewString()` for v4, `uuid.NewV7()` when time-ordered); never hand-roll with `crypto/rand` + `fmt.Sprintf`.

### Go version policy

- Support one version behind the latest stable Go release (e.g., if Go 1.27 is the latest, CI tests 1.26 and 1.27 and the other jobs run on 1.26); do not depend on features only in the latest.
- The `go` directive in `go.mod` is **not** bumped along with CI: sakumock is embedded as a library, so raise it only when a dependency or a needed feature requires it. `go fix ./...` (run before every commit) respects the directive, so it introduces no newer syntax until then.

### OpenAPI specs

- Each service keeps its spec under `openapi/`, copied from the SDK module by `make openapi` (rerun whenever the SDK dependency is upgraded). Handlers must conform to it (paths, methods, schemas, status codes).
- **Spec-expressible request-body constraints are never hand-written in handlers.** `internal/genvalidate` (see its package doc for spec handling, `-mapping`, `-responses`) generates `bodySchemas` and `responseSchemas` into `validate_gen.go`; `NewHandler` wires `core.NewBodyValidator(bodySchemas, writeError)` and `core.NewResponseValidator(responseSchemas, logger)`, and `routeTable()` wraps every route with the request validator inside the rate limiter and the response validator **innermost** (only what the handler writes is checked). Referential checks and domain logic stay in handlers. Follow the monitoringsuite pattern when wiring a service.
- Regeneration: `make openapi` chains `make generate`; the root `make generate` runs `go generate ./...`. CI's `generated-code` job fails on any diff — commit the regenerated file together with the spec. `validate_gen_test.go` asserts every table key matches a `routeTable()` entry so spec/route drift fails tests.
- Empty and syntax-error bodies bypass the validator so `core.ReadJSON` stays authoritative.
- Response violations never alter the client's response: they are logged at Warn and exposed via `Server.SpecViolations()` and `GET`/`DELETE /_sakumock/spec-violations` (`core.SpecViolationRoutes`). Every service's tests assert zero violations through a `closeAndCheck` (or shared `newServer`) helper — always close test servers through it, since `go test` swallows the WARN logs. Fix handlers rather than weakening schemas.
- **Spec gaps** (constraints the real API enforces but the spec omits — typically a required string with no `minLength`) are declared in `validate_overrides.go` (`bodyNonEmptyFields`, dotted paths, `[]` steps into array items) via `core.WithNonEmpty`; an entry that no longer resolves panics at construction. Remove it once the upstream spec catches up. Anything else (cross-field checks, allow-lists, referential lookups) stays hand-written.
- Error responses conform to the spec's error schema when one exists: `commonserviceitem`-based endpoints use the standard `Error` envelope via `core.WriteStandardError`. Without a spec error schema, match the real API's behavior rather than inventing a shape.

### Code style

- Logging: `log/slog`. Per-request logs MUST be **Info** (the mock's purpose is to show it is handling requests); Debug for internals. Wrap the ResponseWriter with `core.NewResponseRecorder` and log via `core.RequestLogArgs(r, rw, extras...)` so error responses carry their reason.
- CLI: `alecthomas/kong`.
- JSON bodies: a named struct with `json` tags for any fixed shape, even if used once; `map[string]any` only for genuinely dynamic data; share a type when a shape recurs.
- Tests: use the real SAKURA Cloud SDK client against `NewTestServer`.
- End-to-end Terraform test: `test/terraform/` drives the real `sakumock all` binary with the `sacloud/sakura` provider (apply → plan with no diff → destroy), behind the `terraform` build tag (`go test -tags terraform ./test/terraform/`; CI job `terraform-integration`). Add a new service's resource as `test/terraform/<service>.tf` — it also becomes its section in `sakumock docs terraform`. See `test/terraform/doc.go` for how to list the provider's resources and how to recover from a stale `terraform.tfstate`.
- SDK endpoint: `SAKURA_ENDPOINTS_<SERVICE_KEY>` environment variable.
