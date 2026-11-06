# sakumock/securitycontrol

A Security Control (セキュリティコントロール) compatible mock server for local
development and testing. It implements the project activation, evaluation
rule, and automated action APIs with in-memory storage. Evaluations are never
run and automated actions never fire.

## Install

```bash
go install github.com/sacloud/sakumock/securitycontrol/cmd/sakumock-securitycontrol@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock security-control` accepts the same flags as `sakumock-securitycontrol`.

## Usage

```bash
sakumock-securitycontrol
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `SECURITY_CONTROL_LOCALSERVER_ADDR` | `127.0.0.1:18095` | Listen address |
| `--latency` | `SECURITY_CONTROL_LATENCY` | `0` | Artificial latency added to every API response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `SECURITY_CONTROL_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `SECURITY_CONTROL_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--fault` | `SECURITY_CONTROL_FAULT` | (none) | Inject faults: `CODE:RATE[:PHASE]`, repeatable (see [Fault Injection](../README.md#fault-injection)) |
| `--automated-action-limit` | `SECURITY_CONTROL_AUTOMATED_ACTION_LIMIT` | `10` | Maximum number of automated actions, reported as the activation's `automatedActionLimit` |
| `--debug` | `SECURITY_CONTROL_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `SECURITY_CONTROL_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `SECURITY_CONTROL_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/security-control` client reads the `SAKURA_ENDPOINTS_SECURITY_CONTROL` override and calls the spec's bare paths (`/activation`, `/evaluation_rules`, ...) on it. The override applies whether or not `SAKURA_ZONE` is set:

```bash
export SAKURA_ENDPOINTS_SECURITY_CONTROL=http://localhost:18095
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

The Terraform provider (`sakura_security_control_activation`, `sakura_security_control_evaluation_rule`, `sakura_security_control_automated_action`) uses the same client and override.

## Library usage

```go
import "github.com/sacloud/sakumock/securitycontrol"

// As http.Handler (for custom servers)
handler, err := securitycontrol.NewHandler(securitycontrol.Config{})
if err != nil {
	log.Fatal(err)
}
defer handler.Close()

// As test server (for integration tests)
srv := securitycontrol.NewTestServer(securitycontrol.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/activation` | Activate security control for the project |
| GET | `/activation` | Get the project's activation state |
| PUT | `/activation` | Update the project's activation state |
| GET | `/automated_actions` | List automated actions |
| POST | `/automated_actions` | Create an automated action |
| GET | `/automated_actions/{actionId}` | Get an automated action |
| PUT | `/automated_actions/{actionId}` | Update an automated action |
| DELETE | `/automated_actions/{actionId}` | Delete an automated action |
| GET | `/evaluation_rules` | List evaluation rules |
| GET | `/evaluation_rules/{evaluationRuleId}` | Get an evaluation rule |
| PUT | `/evaluation_rules/{evaluationRuleId}` | Update an evaluation rule |

The mock holds one project:

- **Activation** is a singleton. `GET`/`PUT /activation` answer `404` until
  `POST /activation` has run once; a second `POST` answers `409`. `PUT` can
  turn it off (`isActive: false`) and back on, but never removes it.
- **Evaluation rules** are the 14 rules of the spec's `EvaluationRuleID`
  enum, listed in that order, all present from the start and disabled. Their
  `description` is the rule's detail text from the
  [manual](https://manual.sakura.ad.jp/cloud/controlpanel/eventlog/security-control.html);
  `iamRolesRequired` is always empty. `PUT` stores `rule` (including its
  `parameter`) as sent and requires `rule.evaluationRuleId` to match the path
  (`400` otherwise); an ID outside the enum answers `404`.
- **Automated actions** get a generated numeric ID and `createdAt`. `action`
  is stored as sent. Creating one beyond `--automated-action-limit` answers
  `400`. `DELETE` answers `204` even for an unknown ID, since the spec
  declares no `404` for it.
- Neither evaluation rules nor automated actions require the project to be
  activated.

Lists take `page_size` (default 50, capped at 100) and return `items`,
`totalSize`, and the opaque `nextPageToken` / `prevPageToken`, passed back as
`next` / `prev`. As the spec requires, `next` and `prev` together, a malformed
token, or a token replayed with a different `page_size` answer `400`.

Errors are RFC 7807 problem details (`{"title", "status", "detail"}`) whose
`detail` is the fixed value the spec declares for the status (`invalid`,
`not_found`, `conflict`, ...); the occurrence-specific reason is in `title`.
They are sent as `application/json`, which the spec allows beside
`application/problem+json` and which the SDK decodes.

The spec's discriminated unions (`action`, `rule`) are not validated beyond
being present: the request-body validator generated from the spec leaves
`oneOf` permissive.

## Mock-only endpoints

Endpoints under `/_sakumock/` do not exist in the real SAKURA Cloud API; they observe the mock itself.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/_sakumock/health` | Liveness check (`{"status":"ok"}`) |
| `GET` | `/_sakumock/spec-violations` | List responses that diverged from the OpenAPI spec |
| `DELETE` | `/_sakumock/spec-violations` | Clear the recorded violations |

Every handler response is validated against the OpenAPI spec (status code declared, JSON body matching the response schema). A violation never alters the response the client receives: it is logged at `WARN` level and recorded — deduplicated with a count — for inspection. An empty list after exercising the endpoints you care about means the mock's responses conform to the spec.
