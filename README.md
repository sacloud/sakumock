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

Alternatively, download a prebuilt binary from the [Releases](https://github.com/sacloud/sakumock/releases) page, or use the container image (see [Run with Docker](#run-with-docker)):

```bash
docker pull ghcr.io/sacloud/sakumock:latest
```

### Run

Run every service together in one process. This is the usual way to use sakumock:

```bash
sakumock all
```

Run `sakumock all --help` for flags. Per-service flags keep their defaults and are available with a service prefix (e.g. `--kms-latency`, `--simplemq-addr`).

#### TLS

Pass a certificate and key (`--tls-cert`/`--tls-key`, or `SAKUMOCK_TLS_CERT`/`SAKUMOCK_TLS_KEY`) to serve **every** listener over HTTPS — all control planes and data planes share the one cert (they run on the same host, only the port differs). TLS is enabled only when both files are set; otherwise everything stays plain HTTP. The object storage data plane is served by versitygw, which is handed the same cert/key (`--cert`/`--key`) so it terminates TLS itself. Standalone subcommands accept the same `--tls-cert`/`--tls-key` (env `<SERVICE>_TLS_CERT` / `<SERVICE>_TLS_KEY`). `sakumock env` emits `https://` endpoints when TLS is set; with a self-signed cert the client must be told to trust it.

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

#### Environment variables

Every per-service setting also has an environment variable, which is the most convenient way to configure the mock in a container. The names are `<SERVICE>_<SETTING>` (e.g. `KMS_LATENCY`, `SIMPLEMQ_RATE_LIMIT`, `MONITORINGSUITE_DEBUG`); each flag's exact variable is shown in `sakumock all --help` as `($VAR)`. They apply under `sakumock all` just as they do for the standalone subcommands:

```bash
KMS_LATENCY=200ms SIMPLEMQ_RATE_LIMIT=10 sakumock all
```

### Connect your application

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
`--host` to substitute the host the client actually uses (the port is kept), and
`--output FILE` to write a file where shell redirection is unavailable:

```bash
# Endpoints pointing at a host reachable as `localhost`
sakumock env --host localhost > sakumock.env
```

Or set them by hand:

```bash
# SimpleMQ (control plane + message plane share one address)
export SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE=http://localhost:18080
export SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://localhost:18080
# SecretManager
export SAKURA_ENDPOINTS_SECRETMANAGER=http://localhost:18082
# KMS
export SAKURA_ENDPOINTS_KMS=http://localhost:18081
# Simple Notification
export SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=http://localhost:18083
# Monitoring Suite
export SAKURA_ENDPOINTS_MONITORING_SUITE=http://localhost:18084
# EventBus
export SAKURA_ENDPOINTS_EVENTBUS=http://localhost:18085/
# Object Storage (control plane only; no S3 data plane)
export SAKURA_ENDPOINTS_OBJECT_STORAGE=http://localhost:18086
# IAM
export SAKURA_ENDPOINTS_IAM=http://localhost:18087
# AppRun
export SAKURA_ENDPOINTS_APPRUN_SHARED=http://localhost:18088

# Dummy credentials (required by SDK, not validated by mock)
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

### Run a single service

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
```

Run `sakumock --help` to list services, and `sakumock <service> --help` for its flags.

### Run with Docker

A multi-platform image (`linux/amd64`, `linux/arm64`) is published to GitHub Container Registry. Its default command runs every service bound to `0.0.0.0`, so published ports are reachable from the host:

```bash
docker run --rm \
  -p 18080:18080 -p 18081:18081 -p 18082:18082 -p 18083:18083 -p 18084:18084 -p 18085:18085 -p 18086:18086 -p 18087:18087 -p 18088:18088 \
  ghcr.io/sacloud/sakumock:latest
```

A second tag, `:latest-dataplane` (and `:<version>-dataplane`), enables every service's data plane by default: the Object Storage S3 data plane (bundling the [versitygw](https://github.com/versity/versitygw) S3 gateway; `OBJECT_STORAGE_ENABLE_DATA_PLANE=true`, listening on `0.0.0.0:28086`) and the Monitoring Suite telemetry ingest data plane (`MONITORINGSUITE_ENABLE_DATA_PLANE=true`, listening on `0.0.0.0:28084`). The default image includes neither, so use this tag when you need a data plane:

```bash
docker run --rm \
  -p 18080:18080 -p 18081:18081 -p 18082:18082 -p 18083:18083 -p 18084:18084 -p 18085:18085 -p 18086:18086 -p 18087:18087 -p 18088:18088 \
  -p 28084:28084 -p 28086:28086 \
  ghcr.io/sacloud/sakumock:latest-dataplane
```

Objects are stored under `/home/nonroot/data`; mount a volume there to persist them. The image runs as a non-root user (uid 65532), so prefer a **named volume** (`-v sakumock-data:/home/nonroot/data`, which inherits the right ownership) — a bind-mounted host directory must be writable by uid 65532 or the data plane fails to start.

Configure the mock's behavior with the per-service environment variables (see [Environment variables](#environment-variables)) — handier than flags in a container:

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

### Use as a library in tests

Each service can also be used as a Go library to spin up in-process test servers:

```go
import "github.com/sacloud/sakumock/secretmanager"

srv := secretmanager.NewTestServer(secretmanager.Config{})
defer srv.Close()
// srv.TestURL() returns http://127.0.0.1:<random-port>
```

## Module Conventions

Each service package must follow these conventions:

### Public API

| Symbol | Description |
|---|---|
| `Config` | Configuration struct with `alecthomas/kong` tags for CLI parsing |
| `Config.ClientEnv() []core.EnvVar` | The `SAKURA_ENDPOINTS_*` override(s) a client sets to reach this mock, derived from `Config.Addr` |
| `Command` | kong command embedding `Config`; reused by both the standalone binary and the unified `sakumock` binary |
| `NewHandler(cfg Config) (*Server, error)` | Create an `http.Handler` without starting a listener |
| `NewTestServer(cfg Config) *Server` | Create and start an `httptest.Server` for use in tests |
| `Server.TestURL() string` | Return the base URL of the test server |
| `Server.Close()` | Shut down the server and release resources |

`*Server` satisfies the `core.Server` interface and `Config` satisfies `core.ServiceConfig`, both asserted at compile time in each service so the unified binary can build and treat every service uniformly.

### Structure

Each service is a package under its own subdirectory with:

- `store.go` — `Store` interface and domain types
- `store_memory.go` — In-memory `Store` implementation
- `new_store.go` — Store factory function
- `handler.go` — HTTP handlers and JSON request/response types
- `server.go` — `Config`, `Server`, `NewHandler`, `NewTestServer`
- `cli.go` — `Command` (kong command embedding `Config` + `--routes` + `Run`)
- `cmd/sakumock-<service>/` — standalone CLI entrypoint (a thin shim over `Command`)
- `Makefile` — build, test, install targets
- `README.md` — usage and API documentation

The unified `sakumock` binary lives at `cmd/sakumock`; it imports each service's `Command` and registers it as a subcommand. Shared CLI plumbing (graceful shutdown, logging, the serve loop) lives in the `core` package.

### Port Allocation

Default ports are assigned sequentially starting from 18080. See the service table above.

## License

This project is published under [Apache 2.0 License](LICENSE).
