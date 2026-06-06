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

New services should use the next available port in sequence (18084, 18085, ...).

## Quick Start

### Install

Download a prebuilt binary from the [Releases](https://github.com/sacloud/sakumock/releases) page, or install with Go:

```bash
go install github.com/sacloud/sakumock/cmd/sakumock@latest
```

### Run

```bash
# Run every service together in one process (recommended)
sakumock all

# ...or run a single service as a subcommand
sakumock simplemq &
sakumock secretmanager &
sakumock kms &
sakumock simplenotification &
```

Run `sakumock --help` to list services, and `sakumock <service> --help` (or `sakumock all --help`) for flags. Under `all`, per-service flags keep their defaults and are available with a service prefix (e.g. `--kms-latency`, `--simplemq-addr`).

Instead of passing many flags, `sakumock all` can read a config file (`--config`, YAML or JSON by extension) with options grouped per service. CLI flags override the file:

```yaml
# sakumock.yaml
simplemq:
  addr: 127.0.0.1:28080
  database: /var/lib/sakumock/mq.db
kms:
  latency: 5s
```

```bash
sakumock all --config sakumock.yaml
```

### Connect your application

`sakumock all` can write the environment variables your client (SAKURA Cloud SDK or the Terraform provider) needs into a dotenv file, so you never hand-copy endpoints:

```bash
sakumock all --write-env-file ./sakumock.env

# In the shell that runs your SDK / Terraform:
set -a; source ./sakumock.env; set +a
terraform apply
```

The generated file sets each service's `SAKURA_ENDPOINTS_*` override plus dummy
credentials. The dummy credentials also act as a safety net: a request to an API
that sakumock does not mock reaches the real endpoint but fails authentication
instead of touching your account.

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

# Dummy credentials (required by SDK, not validated by mock)
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

### Use as a library in tests

Each service can also be used as a Go library to spin up in-process test servers:

```go
import "github.com/sacloud/sakumock/secretmanager"

srv := secretmanager.NewTestServer(secretmanager.Config{})
defer srv.Close()
// srv.TestURL() returns http://127.0.0.1:<random-port>
```

## Module Conventions

Each service module must follow these conventions:

### Public API

| Symbol | Description |
|---|---|
| `Config` | Configuration struct with `alecthomas/kong` tags for CLI parsing |
| `Command` | kong command embedding `Config`; reused by both the standalone binary and the unified `sakumock` binary |
| `NewHandler(cfg Config) (*Server, error)` | Create an `http.Handler` without starting a listener |
| `NewTestServer(cfg Config) *Server` | Create and start an `httptest.Server` for use in tests |
| `Server.TestURL() string` | Return the base URL of the test server |
| `Server.Close()` | Shut down the server and release resources |

### Structure

Each module is an independent Go module (`go.mod`) under its own subdirectory with:

- `store.go` — `Store` interface and domain types
- `store_memory.go` — In-memory `Store` implementation
- `new_store.go` — Store factory function
- `handler.go` — HTTP handlers and JSON request/response types
- `server.go` — `Config`, `Server`, `NewHandler`, `NewTestServer`
- `cli.go` — `Command` (kong command embedding `Config` + `--routes` + `Run`)
- `cmd/sakumock-<service>/` — standalone CLI entrypoint (a thin shim over `Command`)
- `Makefile` — build, test, install targets
- `README.md` — usage and API documentation

The unified `sakumock` binary lives in the repository-root module at `cmd/sakumock`; it imports each service's `Command` and registers it as a subcommand. Shared CLI plumbing (graceful shutdown, logging, the serve loop) lives in the `core` module.

### Port Allocation

Default ports are assigned sequentially starting from 18080. See the service table above.

## License

This project is published under [Apache 2.0 License](LICENSE).
