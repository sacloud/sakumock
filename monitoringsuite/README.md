# sakumock/monitoringsuite

A Monitoring Suite (モニタリングスイート) compatible mock server for local development and testing. It implements the **control plane** of the monitoring-suite API with in-memory storage: alert projects (with rules, log-measure rules, notification targets and routings), dashboard projects, log/metrics/trace storages (with access keys, usage stats and expiry), log/metrics routings, publishers, and management endpoints (limits, provisioning).

The **data plane** (sending and querying logs, metrics and traces) is intentionally out of scope; this mock covers resource configuration only.

## Install

```bash
go install github.com/sacloud/sakumock/monitoringsuite/cmd/sakumock-monitoringsuite@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock monitoringsuite` accepts the same flags as `sakumock-monitoringsuite`.

## Usage

```bash
sakumock-monitoringsuite
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `MONITORINGSUITE_LOCALSERVER_ADDR` | `127.0.0.1:18084` | Listen address |
| `--latency` | `MONITORINGSUITE_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `MONITORINGSUITE_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `MONITORINGSUITE_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `MONITORINGSUITE_DEBUG` | `false` | Enable debug mode |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/monitoring-suite` client reads the `SAKURA_ENDPOINTS_MONITORING_SUITE` override (service key `monitoring_suite`):

```bash
export SAKURA_ENDPOINTS_MONITORING_SUITE=http://localhost:18084
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/monitoringsuite"

// As http.Handler (for custom servers)
handler, _ := monitoringsuite.NewHandler(monitoringsuite.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := monitoringsuite.NewTestServer(monitoringsuite.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

Run `sakumock-monitoringsuite --routes` for the full list. The server implements
every control-plane path of the monitoring-suite OpenAPI spec, grouped as:

| Group | Paths |
|-------|-------|
| Alert projects | `/alerts/projects/` (+ `{resource_id}`) |
| Alert rules | `/alerts/projects/{project_resource_id}/rules/` (+ histories) |
| Log-measure rules | `/alerts/projects/{project_resource_id}/log-measure-rules/` |
| Notification targets | `/alerts/projects/{project_resource_id}/notification-targets/` |
| Notification routings | `/alerts/projects/{project_resource_id}/notification-routings/` (+ `reorder`) |
| Dashboard projects | `/dashboards/projects/` (+ `{resource_id}`) |
| Log routings | `/logs/routings/` |
| Log storages | `/logs/storages/` (+ keys, stats, set-expire) |
| Metrics routings | `/metrics/routings/` |
| Metrics storages | `/metrics/storages/` (+ keys, stats) |
| Trace storages | `/traces/storages/` (+ keys, stats, set-expire) |
| Publishers | `/publishers/` (read-only) |
| Management | `/management/limits/`, `/management/provisioning/` |

## OpenAPI spec

`openapi/openapi.json` is copied from the `github.com/sacloud/sacloud-sdk-go`
module (`api/monitoring-suite/openapi`). Run `make openapi` to refresh it after
upgrading the SDK dependency.
