# sakumock

**Status: Alpha (under active development).** APIs, behavior, and module layout may change without notice. Not recommended for production use.

A local mock server suite for SAKURA Cloud APIs, inspired by [LocalStack](https://github.com/localstack/localstack). Run SAKURA Cloud services locally for development and testing without connecting to the real API.

## Services

Every service is available as a subcommand of the single `sakumock` binary (e.g. `sakumock simplemq`), and as a Go library for spinning up in-process test servers. See each service's README for details.

| Service | Default Port | Module | Description |
|---|---|---|---|
| [simplemq](simplemq/) | 18080 | `github.com/sacloud/sakumock/simplemq` | SimpleMQ message API |
| [secretmanager](secretmanager/) | 18082 | `github.com/sacloud/sakumock/secretmanager` | SecretManager API |
| [kms](kms/) | 18081 | `github.com/sacloud/sakumock/kms` | KMS key management API |
| [simplenotification](simplenotification/) | 18083 | `github.com/sacloud/sakumock/simplenotification` | Simple Notification message-send API |
| [monitoringsuite](monitoringsuite/) | 18084 | `github.com/sacloud/sakumock/monitoringsuite` | Monitoring Suite control-plane API |
| [eventbus](eventbus/) | 18085 | `github.com/sacloud/sakumock/eventbus` | EventBus control-plane API |
| [objectstorage](objectstorage/) | 18086 | `github.com/sacloud/sakumock/objectstorage` | Object Storage control-plane API (optional S3 data plane via versitygw) |
| [iam](iam/) | 18087 | `github.com/sacloud/sakumock/iam` | IAM control-plane API |
| [apprun](apprun/) | 18088 | `github.com/sacloud/sakumock/apprun` | AppRun control-plane API (optional Docker data plane) |
| [apprundedicated](apprundedicated/) | 18089 | `github.com/sacloud/sakumock/apprundedicated` | AppRun Dedicated control-plane API (optional Docker data plane) |
| [workflows](workflows/) | 18090 | `github.com/sacloud/sakumock/workflows` | Workflows control-plane API (optional Runbook execution engine) |
| [apigw](apigw/) | 18091 | `github.com/sacloud/sakumock/apigw` | API Gateway control-plane API (optional gateway data plane) |
| [cloudhsm](cloudhsm/) | 18092 | `github.com/sacloud/sakumock/cloudhsm` | CloudHSM control-plane API |
| [addon](addon/) | 18093 | `github.com/sacloud/sakumock/addon` | Add-on API (AI / CDN / security / data analytics resources) |

## Quick Start

### Install

Install with [mise](https://mise.jdx.dev/), which fetches the prebuilt binary straight from the GitHub Releases:

```bash
# Install globally (latest release)
mise use -g github:sacloud/sakumock@latest
```

Or pin a version in your project's `mise.toml`:

```toml
[tools]
"github:sacloud/sakumock" = "0.2.1"
```

Alternatively, download a prebuilt binary from the [Releases](https://github.com/sacloud/sakumock/releases) page, or use the container image (see [Docker](#docker)):

```bash
docker pull ghcr.io/sacloud/sakumock:latest
```

### Run

Run every service together in one process. This is the usual way to use sakumock:

```bash
sakumock all
```

Run `sakumock all --help` for flags. Per-service flags are available with a service prefix (e.g. `--kms-latency`, `--simplemq-addr`).

### Connect Your Application

The `sakumock env` subcommand prints the environment variables your client (SAKURA Cloud SDK or the Terraform provider) needs as a dotenv file, so you never hand-copy endpoints. It starts no server, so you can run it before (or independently of) `sakumock all`:

```bash
sakumock env > ./sakumock.env

# In the shell that runs your SDK / Terraform:
set -a; source ./sakumock.env; set +a
terraform apply
```

The file sets each service's `SAKURA_ENDPOINTS_*` override plus dummy
credentials. The dummy credentials also act as a safety net: a request to an API
that sakumock does not mock reaches the real endpoint but fails authentication
instead of touching your account.

Pass `--export` to prefix every line with `export `, so the output can be
sourced directly (e.g. with [direnv](https://direnv.net/) or a plain shell)
without `set -a`:

```bash
sakumock env --export > .envrc   # or: source <(sakumock env --export)
```

By default the endpoints point at each service's listen address. When the client
reaches sakumock over the network — most importantly from a container — pass
`--host` to substitute the host the client actually uses (the port is kept):

```bash
sakumock env --host localhost > sakumock.env
```

Run `sakumock env` to see the full list of variables, or `sakumock all --help` for each flag's environment variable.

## Configuration

`sakumock all` accepts per-service flags, a config file, and environment variables.

### Flags

Per-service flags keep their defaults and are available with a service prefix (e.g. `--kms-latency`, `--simplemq-addr`). Run `sakumock all --help` for a full listing.

### Config File

Instead of passing many flags, `sakumock all` can read a config file (`--config`, YAML or JSON by extension) with options grouped per service:

```yaml
# sakumock.yaml
simplemq:
  addr: 127.0.0.1:28080
  database: /var/lib/sakumock/mq.db
  message-expire: 96h
kms:
  latency: 5s
```

```bash
sakumock all --config sakumock.yaml
```

Each per-service flag maps to a key by stripping the service prefix: `--simplemq-message-expire` becomes `message-expire` under `simplemq:`, `--kms-latency` becomes `latency` under `kms:`. Precedence, highest first: command-line flag, then config file, then environment variable, then the flag's default.

### Environment Variables

Every per-service setting also has an environment variable, which is the most convenient way to configure the mock in a container. The names are `<SERVICE>_<SETTING>` (e.g. `KMS_LATENCY`, `SIMPLEMQ_RATE_LIMIT`, `MONITORINGSUITE_DEBUG`); each flag's exact variable is shown in `sakumock all --help` as `($VAR)`. They apply under `sakumock all` just as they do for the standalone subcommands:

```bash
KMS_LATENCY=200ms SIMPLEMQ_RATE_LIMIT=10 sakumock all
```

### Run a Single Service

You can also run any service on its own as a subcommand (same flags, without the service prefix):

```bash
sakumock simplemq &
sakumock secretmanager &
sakumock kms &
sakumock simplenotification &
sakumock monitoringsuite &
sakumock eventbus &
sakumock objectstorage &
sakumock iam &
sakumock apprun &
sakumock apprun-dedicated &
sakumock workflows &
sakumock cloudhsm &
sakumock addon &
```

Run `sakumock --help` to list services, and `sakumock <service> --help` for its flags.

## Fault Injection

To test how a client (SDK, Terraform provider, your application) handles server errors and network failures, every service can probabilistically inject faults into its control-plane API with `--fault CODE:RATE[:PHASE]` (repeatable):

- `CODE` — the HTTP status to return (200–599), or `reset` to drop the TCP connection abruptly (the client sees a transport error such as `connection reset by peer`, not an HTTP response).
- `RATE` — the probability in `(0, 1]`. Each request rolls once; the configured rates are exact and mutually exclusive, so they must sum to at most 1.
- `PHASE` — when the fault fires, relative to the mock's own processing:
  - `before` (default): the request is rejected before the handler runs — no state changes, safe to retry.
  - `after`: the handler runs first (state changes **do** happen), then its response is discarded and replaced by the fault — emulates "the server committed but the client got an error/disconnect", which is what exercises retry idempotency in a client.

```bash
# Standalone: 10% HTTP 500, 2% connection reset
sakumock kms --fault 500:0.1 --fault reset:0.02

# Under sakumock all, use the service prefix
sakumock all --kms-fault 500:0.1 --kms-fault 500:0.1:after

# Env var (comma-separated)
KMS_FAULT=500:0.1,reset:0.02 sakumock all
```

Config file (quote the specs — unquoted `500:0.1` in YAML flow style would parse as a mapping):

```yaml
kms:
  fault:
    - "500:0.1"
    - "reset:0.02:after"
```

An injected status is returned in the service's own error envelope with a `fault injection` message, and the request log records it (`reset` logs as status 499). When [tracing](#opentelemetry-tracing) is enabled, the request's span is annotated with `sakumock.fault.code` and `sakumock.fault.phase` (plus `sakumock.fault.replaced_status` for `after` faults), and a `reset` marks the span status as error — so an injected fault is distinguishable from a real one in traces. Faults fire before authentication, rate limiting, and validation — like an infrastructure failure, an injected fault can mask a would-be 401/429/400. They never apply to the mock-only `/_sakumock/` inspection endpoints, and in this iteration they cover control planes only (the separate-listener data planes — object storage S3, Monitoring Suite ingest, AppRun proxies, API Gateway — are not fault-injected).

## TLS

Pass a certificate and key (`--tls-cert`/`--tls-key`, or `SAKUMOCK_TLS_CERT`/`SAKUMOCK_TLS_KEY`) to serve **every** listener over HTTPS — all control planes and data planes share the one cert (they run on the same host, only the port differs). TLS is enabled only when both files are set; otherwise everything stays plain HTTP. The object storage data plane is served by versitygw, which is handed the same cert/key (`--cert`/`--key`) so it terminates TLS itself. Standalone subcommands accept the same `--tls-cert`/`--tls-key` (env `<SERVICE>_TLS_CERT` / `<SERVICE>_TLS_KEY`). `sakumock env` emits `https://` endpoints when TLS is set; with a self-signed cert the client must be told to trust it.

## Service Link

Pass `--enable-service-link` (or `SAKUMOCK_ENABLE_SERVICE_LINK=true`) to let services forward requests to each other, just as the real SAKURA Cloud platform does. Currently EventBus forwards fired jobs to their destination service:

| Source | Destination | What happens |
|---|---|---|
| EventBus | SimpleMQ | Sends a message to the configured queue |
| EventBus | SimpleNotification | Sends a notification to the configured group |

Without `--enable-service-link` (the default), firings are recorded but not forwarded — each service works in isolation.

Service link requires the source service's data plane to be enabled as well — EventBus needs `--eventbus-enable-data-plane` to fire events:

```bash
sakumock all --enable-service-link --eventbus-enable-data-plane
```

Service link is only available with `sakumock all`; standalone services cannot discover each other's addresses.

## OpenTelemetry Tracing

sakumock can emit OpenTelemetry traces for every request it handles — one server span per request, continuing the client's trace when the request carries a W3C `traceparent` header. Configuration uses the standard OTEL environment variables only; there are no sakumock flags:

```bash
# Export spans to any OTLP/HTTP endpoint (a collector, Jaeger, ...):
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 sakumock all
```

Tracing is enabled when `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set (and `OTEL_SDK_DISABLED` is not `true`); otherwise it is completely off. Spans are exported over OTLP http/protobuf by default; set `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` (or `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=grpc`) to export over gRPC instead (e.g. `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317`).

- Spans are named after the matched route (e.g. `GET /cloud/zone/is1a/api/kms/1.0/keys/{id}`) and carry `http.route` plus a `sakumock.service` attribute identifying the service; the resource is `service.name=sakumock` (override with `OTEL_SERVICE_NAME`).
- Every `request` log line gains `trace_id` and `span_id` attributes, so logs and traces correlate.
- All listeners are instrumented: every control plane and the in-process data planes (monitoring suite ingest, AppRun proxies, API gateway). The object storage S3 data plane (versitygw, an external process) is not.
- The SDK's other standard env vars work as usual: `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_BSP_*`, ...

sakumock can even export to its own Monitoring Suite mock ingest:

```bash
MONITORINGSUITE_ENABLE_DATA_PLANE=true \
MONITORINGSUITE_DATA_PLANE_DUMP_DIR=/tmp/telemetry \
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:28084 sakumock all
```

Note that in this loopback setup each export POST is itself traced, so an idle sakumock emits one heartbeat span per batch interval — harmless, but visible in the dumps.

## Inspection

Every service exposes mock-only `/_sakumock/` endpoints for observing or driving the mock — listing accepted messages, inspecting fired deliveries, injecting events, and resetting state. These endpoints do not exist in the real SAKURA Cloud API.

See each service's README for available endpoints and response shapes:

- [eventbus — Inspection endpoints](eventbus/README.md#api-endpoints) (`/_sakumock/events`, `/_sakumock/tick`, `/_sakumock/deliveries`)
- [simplenotification — Inspection endpoints](simplenotification/README.md#inspection-endpoints) (`/_sakumock/messages`)

### Inspection API (Go)

Each service with inspection endpoints provides an `InspectionClient` — an HTTP client that wraps the `/_sakumock/` calls and returns typed domain objects. Use it from integration tests that run against a sakumock process or container (not just in-process test servers):

```go
import "github.com/sacloud/sakumock/eventbus"

ic := eventbus.NewInspectionClient("http://localhost:18085")
ds, _ := ic.InjectEvent(ctx, eventbus.Event{Source: "//monitoringsuite..."})
ds, _ = ic.Deliveries(ctx)
_ = ic.ClearDeliveries(ctx)
```

### Inspection CLI

The `sakumock inspect` subcommand calls inspection endpoints from the command line:

```bash
# List recorded firings
sakumock inspect eventbus deliveries

# Inject an event and see which triggers fired
sakumock inspect eventbus inject-event --source "//monitoringsuite..."

# Force schedule evaluation at a specific time
sakumock inspect eventbus tick --at 2024-01-01T09:00:00+09:00

# List accepted notification messages
sakumock inspect simplenotification messages

# Clear state between test runs
sakumock inspect eventbus clear-deliveries
sakumock inspect simplenotification clear-messages
```

Each service subcommand accepts `--addr` to override the server address. The address resolves in order of precedence (highest first): `--addr` flag, then `SAKURA_ENDPOINTS_*` environment variable (so `sakumock env` output works as-is), then the built-in default.

## Docker

A multi-platform image (`linux/amd64`, `linux/arm64`) is published to GitHub Container Registry.

### Basic Usage

The default command runs every service bound to `0.0.0.0`, so published ports are reachable from the host:

```bash
docker run --rm \
  -p 18080:18080 -p 18081:18081 -p 18082:18082 -p 18083:18083 -p 18084:18084 -p 18085:18085 -p 18086:18086 -p 18087:18087 -p 18088:18088 -p 18089:18089 -p 18090:18090 -p 18091:18091 -p 18092:18092 \
  ghcr.io/sacloud/sakumock:latest
```

### Data Plane Image

A second tag, `:latest-dataplane` (and `:<version>-dataplane`), enables every service's data plane by default: the Object Storage S3 data plane (bundling the [versitygw](https://github.com/versity/versitygw) S3 gateway; `OBJECT_STORAGE_ENABLE_DATA_PLANE=true`, listening on `0.0.0.0:28086`), the Monitoring Suite telemetry ingest data plane (`MONITORINGSUITE_ENABLE_DATA_PLANE=true`, listening on `0.0.0.0:28084`), the AppRun / AppRun Dedicated Docker data planes (`APPRUN_ENABLE_DATA_PLANE=true` on `0.0.0.0:28088`, `APPRUN_DEDICATED_ENABLE_DATA_PLANE=true` on `0.0.0.0:28089`), and the API Gateway data plane (`APIGW_ENABLE_DATA_PLANE=true` on `0.0.0.0:28091`). The default image includes none of these, so use this tag when you need a data plane:

```bash
docker run --rm \
  -p 18080:18080 -p 18081:18081 -p 18082:18082 -p 18083:18083 -p 18084:18084 -p 18085:18085 -p 18086:18086 -p 18087:18087 -p 18088:18088 -p 18089:18089 -p 18090:18090 -p 18091:18091 -p 18092:18092 \
  -p 28084:28084 -p 28086:28086 -p 28091:28091 \
  ghcr.io/sacloud/sakumock:latest-dataplane
```

Objects are stored under `/home/nonroot/data`; mount a volume there to persist them. The image runs as a non-root user (uid 65532), so prefer a **named volume** (`-v sakumock-data:/home/nonroot/data`, which inherits the right ownership) — a bind-mounted host directory must be writable by uid 65532 or the data plane fails to start.

### AppRun Data Planes

The AppRun and AppRun Dedicated data planes start Docker containers for each deployed application and reverse-proxy traffic to them. This requires the host Docker socket and `--network host`:

```bash
docker run --rm --network host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/sacloud/sakumock:latest-dataplane
```

- **`-v /var/run/docker.sock:/var/run/docker.sock`** — lets the mock start and stop sibling containers on the host Docker daemon.
- **`--network host`** — the reverse proxy connects to containers via `127.0.0.1:<port>`, so the mock must share the host's network namespace to reach the published ports. With `--network host` the `-p` flags are unnecessary (all ports are directly accessible).
- The host Docker socket must be readable by the container's user. Add the host's docker group GID with `--group-add $(stat -c '%g' /var/run/docker.sock)` (or `chmod 666 /var/run/docker.sock` on the host as a less secure alternative).

If you do not need the AppRun data planes, disable them with environment variables to skip the Docker dependency:

```bash
docker run --rm \
  -e APPRUN_ENABLE_DATA_PLANE=false \
  -e APPRUN_DEDICATED_ENABLE_DATA_PLANE=false \
  -p 18080:18080 -p 18081:18081 -p 18082:18082 -p 18083:18083 -p 18084:18084 -p 18085:18085 -p 18086:18086 -p 18087:18087 -p 18088:18088 -p 18089:18089 -p 18090:18090 -p 18091:18091 -p 18092:18092 \
  -p 28084:28084 -p 28086:28086 -p 28091:28091 \
  ghcr.io/sacloud/sakumock:latest-dataplane
```

### Configuration and Client Env

Configure the mock's behavior with the per-service environment variables (see [Environment Variables](#environment-variables)) — handier than flags in a container:

```bash
docker run --rm -p 18081:18081 \
  -e KMS_LATENCY=200ms -e KMS_RATE_LIMIT=10 \
  ghcr.io/sacloud/sakumock:latest
```

Writing an env file *inside* the container is not useful: a file there is invisible to a client on the host, and the in-container listen host is not how the client reaches the service. Instead, generate the client env with the `env` subcommand, telling it the host the client uses (the host shell does the redirection):

```bash
docker run --rm ghcr.io/sacloud/sakumock:latest env --host localhost > sakumock.env
set -a; source ./sakumock.env; set +a
terraform apply
```

For docker compose, run `env` as a oneshot that writes the file into a shared volume (the image has no shell, so use `--output` rather than `>`) and have your app load it. See [`examples/compose.yaml`](examples/compose.yaml) for a complete example.

## Use as a Library

Each service can also be used as a Go library to spin up in-process test servers:

```go
import "github.com/sacloud/sakumock/secretmanager"

srv := secretmanager.NewTestServer(secretmanager.Config{})
defer srv.Close()
// srv.TestURL() returns http://127.0.0.1:<random-port>
```

## Contributing

See [CLAUDE.md](CLAUDE.md) for module conventions, file structure, public API contracts, port allocation, and the architectural guidelines each service must follow.

## License

This project is published under [Apache 2.0 License](LICENSE).
