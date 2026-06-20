# sakumock/apprun

An AppRun Shared (共用型) compatible mock server for local development and testing. It implements the **control plane** of the AppRun shared API with in-memory storage: user management, application CRUD with automatic version tracking, traffic distribution, and packet filter management.

An optional **data plane** can be enabled (`--enable-data-plane`) that launches Docker containers for each created application and reverse-proxies requests via host-based routing (`<app-id>.localhost:<port>`). See [Data plane (Docker reverse proxy)](#data-plane-docker-reverse-proxy) below.

## Install

```bash
go install github.com/sacloud/sakumock/apprun/cmd/sakumock-apprun@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock apprun` accepts the same flags as `sakumock-apprun`.

## Usage

```bash
sakumock-apprun
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `APPRUN_LOCALSERVER_ADDR` | `127.0.0.1:18088` | Listen address |
| `--latency` | `APPRUN_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `APPRUN_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `APPRUN_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `APPRUN_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `APPRUN_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, all listeners (control plane and data plane) serve HTTPS instead of plain HTTP |
| `--tls-key` | `APPRUN_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |
| `--enable-data-plane` | `APPRUN_ENABLE_DATA_PLANE` | `false` | Enable data plane (reverse proxy to Docker containers) |
| `--data-plane-addr` | `APPRUN_DATA_PLANE_ADDR` | `127.0.0.1:28088` | Listen address for the data plane (control-plane port + 10000) |

(Under the unified binary these are prefixed, e.g. `--apprun-enable-data-plane`.)

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/apprun` client reads the `SAKURA_ENDPOINTS_APPRUN_SHARED` override (service key `apprun_shared`):

```bash
export SAKURA_ENDPOINTS_APPRUN_SHARED=http://localhost:18088
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/apprun"

// As http.Handler (for custom servers)
handler, _ := apprun.NewHandler(apprun.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := apprun.NewTestServer(apprun.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

Run `sakumock-apprun --routes` for the full list. The server implements
every control-plane path of the AppRun shared OpenAPI spec:

| Group | Paths |
|-------|-------|
| User | `GET /user`, `POST /user` |
| Applications | `GET /applications`, `POST /applications`, `GET /applications/{id}`, `PATCH /applications/{id}`, `DELETE /applications/{id}` |
| Application status | `GET /applications/{id}/status` |
| Versions | `GET /applications/{id}/versions`, `GET /applications/{id}/versions/{version_id}`, `DELETE /applications/{id}/versions/{version_id}` |
| Version status | `GET /applications/{id}/versions/{version_id}/status` |
| Traffic | `GET /applications/{id}/traffics`, `PUT /applications/{id}/traffics` |
| Packet filter | `GET /applications/{id}/packet_filter`, `PATCH /applications/{id}/packet_filter` |

## Data plane (Docker reverse proxy)

With `--enable-data-plane`, a second listener (default `127.0.0.1:28088`, the control-plane port + 10000) reverse-proxies requests to Docker containers that are automatically managed based on the application's first component image.

**How it works:**

1. When an application is created or updated via the API, sakumock starts a Docker container from the first component's container image with a random host port.
2. When an application is deleted, sakumock stops and removes the container.
3. Requests to `<app-id>.localhost:<data-plane-port>` are proxied to the container's mapped port.

**Requirements:** Docker must be installed and the `docker` CLI must be on `PATH`.

**Host-based routing** uses `*.localhost` (RFC 6761), which resolves to loopback without any DNS configuration. Access your application at:

```
http://<app-id>.localhost:28088
```

This works in browsers and with `curl` (which resolves `*.localhost` to `127.0.0.1`).

**Container naming:** containers are named `sakumock-apprun-<app-id>`. All tracked containers are cleaned up when the server shuts down.

### Differences from the real AppRun

The mock data plane runs plain Docker containers, whereas the real AppRun runs on Knative/Kubernetes. This produces the following observable differences:

**Container environment variables:**

| Variable | Real AppRun | Mock |
|---|---|---|
| `HOSTNAME` | `app-<app-id>-<ver>-deployment-<hash>` (Knative Pod name) | Docker container ID |
| `K_SERVICE`, `K_CONFIGURATION`, `K_REVISION` | Set by Knative | Not present |
| `KUBERNETES_*` | Present (empty) | Not present |

User-defined env vars (set via the `env` field in components) and `PORT` are identical in both.

**Request headers forwarded to the container:**

| Header | Real AppRun | Mock |
|---|---|---|
| `X-Real-Ip` | Client IP | Client IP |
| `X-Request-Id` | UUID | UUID |
| `X-Forwarded-For` | Client IP chain | `127.0.0.1` |
| `Forwarded` | `for=<ip>;host=<host>;proto=https` | Not present |
| `X-Forwarded-Host`, `X-Forwarded-Port`, `X-Forwarded-Proto` | Set by Traefik | Not present |
| `Via`, `K-Proxy-Request`, `X-Forwarded-Server` | Set by LB / Knative activator | Not present |

`X-Real-Ip` and `X-Request-Id` are set by the mock. The remaining proxy-chain headers are not reproduced because they reflect the real infrastructure (Traefik, Knative activator, ELB) that does not exist locally.

### Limitations

The following features are accepted by the control-plane API (stored and returned in responses) but **not enforced** by the data plane:

- **Traffic splitting** — `PUT /applications/{id}/traffics` stores the traffic distribution, but the data plane always routes all requests to the latest version's container. Weighted routing across multiple version containers is not implemented.
- **Multiple version containers** — Only one container runs per application (the latest). Updating an application stops the old container and starts a new one; previous versions are not kept running.
- **Auto-scaling** — `min_scale`, `max_scale`, and `scale_target_concurrency` are stored but have no effect. Exactly one container instance runs per application.
- **Health probes** — `probe` (HTTP GET liveness check) is stored but not executed. The container is considered healthy as soon as `docker run` succeeds.
- **Container registry authentication** — `username` is stored but private registry credentials are not passed to `docker run`. The image must be pullable without authentication, or pre-pulled locally.
- **Packet filter** — `PATCH /applications/{id}/packet_filter` stores the filter rules, but the data plane does not enforce IP-based access control (all traffic is from localhost).
- **Application / version status** — Always returns `"Healthy"`. No actual health monitoring is performed.

## OpenAPI spec

`openapi/openapi.yaml` is copied from the `github.com/sacloud/sacloud-sdk-go`
module (`api/apprun/openapi`). Run `make openapi` to refresh it after
upgrading the SDK dependency.
