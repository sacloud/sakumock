# sakumock/apprundedicated

An AppRun Dedicated (専有型) compatible mock server for local development and testing. It implements the **control plane** of the AppRun Dedicated API with in-memory storage: cluster management, application/version lifecycle, auto-scaling groups, load balancers, worker nodes, and certificate management.

An optional **data plane** can be enabled (`--enable-data-plane`) that launches Docker containers for each application with an active version, and reverse-proxies requests via host-based routing (`<app-id>.localhost:<port>`). See [Data plane (Docker reverse proxy)](#data-plane-docker-reverse-proxy) below.

## Install

```bash
go install github.com/sacloud/sakumock/apprundedicated/cmd/sakumock-apprundedicated@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock apprun-dedicated` accepts the same flags as `sakumock-apprundedicated`.

## Usage

```bash
sakumock-apprundedicated
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `APPRUN_DEDICATED_LOCALSERVER_ADDR` | `127.0.0.1:18089` | Listen address |
| `--latency` | `APPRUN_DEDICATED_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `APPRUN_DEDICATED_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `APPRUN_DEDICATED_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `APPRUN_DEDICATED_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `APPRUN_DEDICATED_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, all listeners (control plane and data plane) serve HTTPS instead of plain HTTP |
| `--tls-key` | `APPRUN_DEDICATED_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |
| `--enable-data-plane` | `APPRUN_DEDICATED_ENABLE_DATA_PLANE` | `false` | Enable data plane (reverse proxy to Docker containers) |
| `--data-plane-addr` | `APPRUN_DEDICATED_DATA_PLANE_ADDR` | `127.0.0.1:28089` | Listen address for the data plane (control-plane port + 10000) |

(Under the unified binary these are prefixed, e.g. `--apprun-dedicated-enable-data-plane`.)

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/apprun-dedicated` client reads the `SAKURA_ENDPOINTS_APPRUN_DEDICATED` override:

```bash
export SAKURA_ENDPOINTS_APPRUN_DEDICATED=http://localhost:18089
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/apprundedicated"

// As http.Handler (for custom servers)
handler, _ := apprundedicated.NewHandler(apprundedicated.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := apprundedicated.NewTestServer(apprundedicated.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

Run `sakumock-apprundedicated --routes` for the full list. The server implements
every control-plane path of the AppRun Dedicated OpenAPI spec:

| Group | Paths |
|-------|-------|
| Clusters | `POST /clusters`, `GET /clusters`, `GET /clusters/{clusterID}`, `PUT /clusters/{clusterID}`, `DELETE /clusters/{clusterID}` |
| Applications | `POST /applications`, `GET /applications`, `GET /applications/{applicationID}`, `PUT /applications/{applicationID}`, `DELETE /applications/{applicationID}`, `GET /applications/{applicationID}/containers` |
| Versions | `POST /applications/{applicationID}/versions`, `GET /applications/{applicationID}/versions`, `GET /applications/{applicationID}/versions/{version}`, `DELETE /applications/{applicationID}/versions/{version}` |
| Auto Scaling Groups | `POST /clusters/{clusterID}/asg`, `GET /clusters/{clusterID}/asg`, `GET /clusters/{clusterID}/asg/{autoScalingGroupID}`, `DELETE /clusters/{clusterID}/asg/{autoScalingGroupID}` |
| Load Balancers | `POST ...load_balancers`, `GET ...load_balancers`, `GET ...load_balancers/{loadBalancerID}`, `DELETE ...load_balancers/{loadBalancerID}` |
| Load Balancer Nodes | `GET ...load_balancer_nodes`, `GET ...load_balancer_nodes/{loadBalancerNodeID}` |
| Worker Nodes | `GET ...worker_nodes`, `GET ...worker_nodes/{workerNodeID}`, `PUT ...worker_nodes/{workerNodeID}/draining` |
| Certificates | `POST /clusters/{clusterID}/certificates`, `GET /clusters/{clusterID}/certificates`, `GET ...certificates/{certificateID}`, `PUT ...certificates/{certificateID}`, `DELETE ...certificates/{certificateID}` |
| Service Classes | `GET /service_classes/lb`, `GET /service_classes/worker` |

## Data plane (Docker reverse proxy)

With `--enable-data-plane`, a second listener (default `127.0.0.1:28089`, the control-plane port + 10000) reverse-proxies requests to Docker containers that are automatically managed based on the application's active version image.

**How it works:**

1. When an application's `activeVersion` is set via `PUT /applications/{id}`, sakumock starts a Docker container from that version's container image with a random host port.
2. When `activeVersion` is cleared (set to null) or the application is deleted, sakumock stops and removes the container.
3. Requests to `<app-id>.localhost:<data-plane-port>` are proxied to the container's mapped port.

**Requirements:** Docker must be installed and the `docker` CLI must be on `PATH`.

**Host-based routing** uses `*.localhost` (RFC 6761), which resolves to loopback without any DNS configuration. Access your application at:

```
http://<app-id>.localhost:28089
```

This works in browsers and with `curl` (which resolves `*.localhost` to `127.0.0.1`).

**Container naming:** containers are named `sakumock-apprundedicated-<app-id>`. All tracked containers are cleaned up when the server shuts down.

### Differences from the real AppRun Dedicated

The mock data plane runs plain Docker containers, whereas the real AppRun Dedicated manages dedicated clusters with worker nodes. This produces the following observable differences:

- **No real scaling** — one container per application regardless of ASG/min/max settings
- **No real load balancing** — requests go directly to the single container
- **Worker nodes** — created instantly with "healthy" status; no real provisioning
- **LB nodes** — created instantly with "healthy" status; no real provisioning
- **Certificates** — PEM is stored and returned, but not used for TLS termination in the data plane
- **Container placement** — `GET /applications/{id}/containers` returns mock data

### Limitations

The following features are accepted by the control-plane API (stored and returned in responses) but **not enforced** by the data plane:

- **Auto-scaling** — min/max nodes, fixed scale settings are stored but have no effect. Exactly one container instance runs per application.
- **Health checks** — stored but not executed. The container is considered healthy as soon as `docker run` succeeds.
- **Load balancer ports** — port configuration is stored but the data plane always proxies on the container's first exposed port.
- **Container registry authentication** — private registry credentials are not passed to `docker run`. The image must be pullable without authentication, or pre-pulled locally.

## OpenAPI spec

`openapi/openapi.json` is copied from the `github.com/sacloud/sacloud-sdk-go`
module (`api/apprun-dedicated/openapi`). Run `make openapi` to refresh it after
upgrading the SDK dependency.
