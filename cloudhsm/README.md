# sakumock/cloudhsm

A CloudHSM (hardware security module) compatible mock server for local development and testing. It implements the CloudHSM partition, client certificate, IPsec peer, and software license APIs (full CRUD where the real API supports it) with in-memory storage.

## Install

```bash
go install github.com/sacloud/sakumock/cloudhsm/cmd/sakumock-cloudhsm@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock cloudhsm` accepts the same flags as `sakumock-cloudhsm`.

## Usage

```bash
sakumock-cloudhsm
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `CLOUDHSM_LOCALSERVER_ADDR` | `127.0.0.1:18092` | Listen address |
| `--latency` | `CLOUDHSM_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `CLOUDHSM_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `CLOUDHSM_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--fault` | `CLOUDHSM_FAULT` | (none) | Inject faults: `CODE:RATE[:PHASE]`, repeatable (see [Fault Injection](../README.md#fault-injection)) |
| `--debug` | `CLOUDHSM_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `CLOUDHSM_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `CLOUDHSM_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/cloudhsm` client reads the `SAKURA_ENDPOINTS_CLOUDHSM` override:

```bash
export SAKURA_ENDPOINTS_CLOUDHSM=http://localhost:18092
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

Unlike KMS/SecretManager (global services, where the override replaces the entire API root URL), the `cloudhsm-api-go` client always joins `<endpoint>/<zone>/api/cloud/1.1/` onto the endpoint regardless of whether it came from an override — CloudHSM hardware is physically zone-scoped in the real product (default zone `is1b` unless `SAKURA_ZONE` is set). The mock accepts any zone segment, so no extra configuration is needed.

## Library usage

```go
import "github.com/sacloud/sakumock/cloudhsm"

// As http.Handler (for custom servers)
handler := cloudhsm.NewHandler(cloudhsm.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := cloudhsm.NewTestServer(cloudhsm.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

All paths are served under `/{zone}/api/cloud/1.1` (see above); the table below omits that common prefix.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/cloudhsm/cloudhsms` | List all CloudHSMs |
| POST | `/cloudhsm/cloudhsms` | Create a new CloudHSM |
| GET | `/cloudhsm/cloudhsms/{resource_id}` | Get a CloudHSM |
| PUT | `/cloudhsm/cloudhsms/{resource_id}` | Update a CloudHSM |
| DELETE | `/cloudhsm/cloudhsms/{resource_id}` | Delete a CloudHSM |
| GET | `/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients` | List clients of a CloudHSM |
| POST | `/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients` | Create a client of a CloudHSM |
| GET | `/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}` | Get a client |
| PUT | `/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}` | Update a client |
| DELETE | `/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}` | Delete a client |
| GET | `/cloudhsm/cloudhsms/{resource_id}/peers` | List peers of a CloudHSM |
| POST | `/cloudhsm/cloudhsms/{resource_id}/peers` | Create a peer of a CloudHSM |
| DELETE | `/cloudhsm/cloudhsms/{resource_id}/peers/{peer_id}` | Delete a peer |
| GET | `/cloudhsm/licenses` | List all CloudHSM software licenses |
| POST | `/cloudhsm/licenses` | Create a new CloudHSM software license |
| GET | `/cloudhsm/licenses/{resource_id}` | Get a CloudHSM software license |
| PUT | `/cloudhsm/licenses/{resource_id}` | Update a CloudHSM software license |
| DELETE | `/cloudhsm/licenses/{resource_id}` | Delete a CloudHSM software license |

Peers have no update API (create/list/delete only). Creating a peer returns `204 No Content` (no body), matching the real API; listing peers returns a bare `{"Peers": [...]}` array without pagination fields, unlike the other list endpoints.

## Mock-only endpoints

Endpoints under `/_sakumock/` do not exist in the real SAKURA Cloud API; they observe the mock itself.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/_sakumock/spec-violations` | List responses that diverged from the OpenAPI spec |
| `DELETE` | `/_sakumock/spec-violations` | Clear the recorded violations |

Every handler response is validated against the OpenAPI spec (status code declared, JSON body matching the response schema). A violation never alters the response the client receives: it is logged at `WARN` level and recorded — deduplicated with a count — for inspection. An empty list after exercising the endpoints you care about means the mock's responses conform to the spec.
