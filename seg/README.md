# sakumock/seg

A Service Endpoint Gateway compatible mock server for local development and
testing. It implements the appliance CRUD, configuration-apply, power
control, and interface-inspection APIs with in-memory storage.

## Install

```bash
go install github.com/sacloud/sakumock/seg/cmd/sakumock-seg@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock seg` accepts the same flags as `sakumock-seg`.

## Usage

```bash
sakumock-seg
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `SEG_LOCALSERVER_ADDR` | `127.0.0.1:18093` | Listen address |
| `--latency` | `SEG_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `SEG_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `SEG_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--fault` | `SEG_FAULT` | (none) | Inject faults: `CODE:RATE[:PHASE]`, repeatable (see [Fault Injection](../README.md#fault-injection)) |
| `--debug` | `SEG_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `SEG_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `SEG_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/service-endpoint-gateway` client reads the `SAKURA_ENDPOINTS_SERVICE_ENDPOINT_GATEWAY` override:

```bash
export SAKURA_ENDPOINTS_SERVICE_ENDPOINT_GATEWAY=http://localhost:18093
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

**`SAKURA_ZONE` must stay unset.** Unlike `cloudhsm-api-go` (which appends
`/<zone>/api/cloud/1.1/` *on top of* the override, so any zone segment is
accepted), the `service-endpoint-gateway-api-go` client **discards the
override entirely** whenever a zone is configured and rebuilds a hardcoded
`https://secure.sakura.ad.jp/cloud/zone/<zone>/api/cloud/1.1/` URL instead —
this is an upstream SDK quirk, not something the mock can work around. Only
when `SAKURA_ZONE` is unset (and no zone comes from a profile) does the
override apply, and it applies with **no added path prefix**: the client
calls the spec's bare paths (`/appliance`, `/appliance/{id}`, ...) directly
against the override host, which is exactly what this mock serves.

## Library usage

```go
import "github.com/sacloud/sakumock/seg"

// As http.Handler (for custom servers)
handler, err := seg.NewHandler(seg.Config{})
if err != nil {
	log.Fatal(err)
}
defer handler.Close()

// As test server (for integration tests)
srv := seg.NewTestServer(seg.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/appliance` | List all Service Endpoint Gateways |
| POST | `/appliance` | Create a new Service Endpoint Gateway |
| GET | `/appliance/{applianceID}` | Get a Service Endpoint Gateway |
| PUT | `/appliance/{applianceID}` | Update a Service Endpoint Gateway |
| DELETE | `/appliance/{applianceID}` | Delete a Service Endpoint Gateway |
| PUT | `/appliance/{applianceID}/config` | Apply a Service Endpoint Gateway's configuration |
| GET | `/appliance/{applianceID}/interface/{interfaceID}` | Get a Service Endpoint Gateway interface |
| GET | `/appliance/{applianceID}/power` | Get a Service Endpoint Gateway's power status |
| PUT | `/appliance/{applianceID}/power` | Power on a Service Endpoint Gateway |
| DELETE | `/appliance/{applianceID}/power` | Power off a Service Endpoint Gateway |
| PUT | `/appliance/{applianceID}/reset` | Reset a Service Endpoint Gateway's power status |

Creating an appliance responds `202 Accepted` (per the spec); every other
endpoint responds `200 OK`. The mock keeps the lifecycle synchronous for
simplicity:

- **Create auto-powers-on**: the returned appliance is immediately
  `Availability: "available"` with `Instance.Status: "up"` — there is no
  `migrating`/`down` transient state to poll for.
- **Update applies immediately.** The real API requires a separate call to
  `PUT .../config` to "apply" settings written by `PUT /appliance/{id}`; the
  mock applies `Update` directly (recomputing `SettingsHash`) and treats
  `PUT .../config` as a no-op that just re-returns the current state.
- Power on/off/reset are idempotent; there is no precondition check (e.g.
  "must power off before delete").
- Each appliance always exposes exactly two interfaces: a `shared`-scope one
  with a synthetic `IPAddress`, and a `user`-scope one whose `Switch.ID` is
  the switch ID given at creation and whose `UserIPAddress` is the first
  `Remark.Servers[].IPAddress` given at creation. `GET .../interface/{id}`
  matches `interfaceID` against either interface's `Switch.ID`.

## Mock-only endpoints

Endpoints under `/_sakumock/` do not exist in the real SAKURA Cloud API; they observe the mock itself.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/_sakumock/spec-violations` | List responses that diverged from the OpenAPI spec |
| `DELETE` | `/_sakumock/spec-violations` | Clear the recorded violations |

Every handler response is validated against the OpenAPI spec (status code declared, JSON body matching the response schema). A violation never alters the response the client receives: it is logged at `WARN` level and recorded — deduplicated with a count — for inspection. An empty list after exercising the endpoints you care about means the mock's responses conform to the spec.
