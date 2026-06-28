# sakumock/monitoringsuite

A Monitoring Suite (モニタリングスイート) compatible mock server for local development and testing. It implements the **control plane** of the monitoring-suite API with in-memory storage: alert projects (with rules, log-measure rules, notification targets and routings), dashboard projects, log/metrics/trace storages (with access keys, usage stats and expiry), log/metrics routings, publishers, and management endpoints (limits, provisioning).

An optional **telemetry data plane** can be enabled (`--enable-data-plane`) that accepts metric/log/trace ingestion — Prometheus remote-write (metrics) and OTLP/HTTP (logs, traces). It is **ingest only**: payloads are acknowledged (and optionally logged or dumped as JSON for debugging), not stored or queryable. The query side remains out of scope. See [Data plane (telemetry ingest)](#data-plane-telemetry-ingest) below.

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
| `--tls-cert` | `MONITORINGSUITE_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, all listeners (control plane and data plane) serve HTTPS instead of plain HTTP |
| `--tls-key` | `MONITORINGSUITE_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |
| `--enable-data-plane` | `MONITORINGSUITE_ENABLE_DATA_PLANE` | `false` | Serve the telemetry data plane (Prometheus remote-write + OTLP/HTTP); ingest only |
| `--data-plane-addr` | `MONITORINGSUITE_DATA_PLANE_ADDR` | `127.0.0.1:28084` | Listen address for the data plane (control-plane port + 10000) |
| `--data-plane-dump-dir` | `MONITORINGSUITE_DATA_PLANE_DUMP_DIR` | (none) | Write each received payload as JSON to this directory; empty disables file dumps. Combine with `--debug` to also log payloads |

(Under the unified binary these are prefixed, e.g. `--monitoringsuite-enable-data-plane`.)

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
| Alert projects | `/alerts/projects/` (+ `{resource_id}`, histories) |
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

## Data plane (telemetry ingest)

With `--enable-data-plane`, a second listener (default `127.0.0.1:28084`, the control-plane port + 10000) accepts telemetry ingestion and acknowledges it. It is **ingest only** — there is no query/read side.

| Method | Path | Signal | Success |
|--------|------|--------|---------|
| POST | `/prometheus/api/v1/write` | metrics (Prometheus remote-write) | `204` |
| POST | `/v1/logs` | logs (OTLP/HTTP) | `200` |
| POST | `/v1/traces` | traces (OTLP/HTTP) | `200` |

This mirrors the `sacloud` exporter's data flow — metrics as remote-write, logs/traces as OTLP/HTTP — so there is no OTLP `/v1/metrics` endpoint.

Point any compatible client at the endpoint — e.g. the [sacloud-otel-collector](https://github.com/sacloud/sacloud-otel-collector) (or any OpenTelemetry SDK / Prometheus remote-write client) configured to export here instead of to SAKURA Cloud. Its SAKURA-specific `sacloud` exporter accepts plain `http://` endpoints (v0.7.0+), so just point it at `http://<data-plane-addr>`.

The end-to-end test (`go test -tags otelcollector ./monitoringsuite/`) runs the real `sacloud-otel-collector` with `telemetrygen` as the source and asserts the dumps — the recommended way to verify compatibility (it skips unless both binaries are on `PATH`; CI installs them). OTLP bodies may be gzip-compressed (the `sacloud` exporter gzips logs/traces); the mock decompresses them transparently.

Every request is decoded to validate its wire format, so a malformed body is rejected with `400` instead of being acked — the mock checks what is actually sent. Decoding uses a hand-rolled protobuf-wire parser for remote-write and the OTLP protobuf types for OTLP, deliberately avoiding gRPC and the full Prometheus/Collector dependency trees so the binary stays lean.

On top of that validation, you can inspect what was sent:

- `--debug` logs a decoded summary per request, and the full payload as JSON at debug level.
- `--data-plane-dump-dir DIR` writes each received payload as a JSON file (`<signal>-NNNNNN.json`) — remote-write is decoded to `{timeseries:[{labels,samples}]}`, OTLP to its standard JSON representation.

## OpenAPI spec

`openapi/openapi.json` is copied from the `github.com/sacloud/sacloud-sdk-go`
module (`api/monitoring-suite/openapi`). Run `make openapi` to refresh it after
upgrading the SDK dependency.
