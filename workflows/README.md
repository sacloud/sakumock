# sakumock/workflows

A Workflows compatible mock server for local development and testing. It implements the workflow management API (CRUD, revisions, executions, plans, subscriptions) with in-memory storage.

Executions complete immediately with status `Succeeded` (no actual runbook execution engine).

## Install

```bash
go install github.com/sacloud/sakumock/workflows/cmd/sakumock-workflows@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock workflows` accepts the same flags as `sakumock-workflows`.

## Usage

```bash
sakumock-workflows
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `WORKFLOWS_LOCALSERVER_ADDR` | `127.0.0.1:18090` | Listen address |
| `--latency` | `WORKFLOWS_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `WORKFLOWS_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `WORKFLOWS_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `WORKFLOWS_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `WORKFLOWS_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `WORKFLOWS_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/workflows` client reads the `SAKURA_ENDPOINTS_WORKFLOWS` override:

```bash
export SAKURA_ENDPOINTS_WORKFLOWS=http://localhost:18090
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/workflows"

// As http.Handler (for custom servers)
handler, err := workflows.NewHandler(workflows.Config{})
if err != nil {
    panic(err)
}
defer handler.Close()

// As test server (for integration tests)
srv := workflows.NewTestServer(workflows.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## Expression evaluator safety limits

The inline expression evaluator (`${...}`) enforces the following limits to prevent DoS from user-supplied expressions:

| Limit | Default | Description |
|-------|---------|-------------|
| Parse depth | 128 | Maximum nesting depth of parentheses/operators in a single expression |
| Evaluation steps | 100,000 | Maximum number of AST node evaluations per expression |
| Array length | 1,000,000 | Maximum number of elements `array.range` can generate |

Regular expressions (`text.findAllRegex`, `text.matchRegex`, `text.replaceAllRegex`) use Go's `regexp` package (RE2 semantics), which guarantees linear-time matching and is not susceptible to ReDoS.

`text.decode` / `text.encode` only support UTF-8. Passing a non-UTF-8 charset returns an error.

## HTTP call safety limits

The `call` step's HTTP functions (`http.get`, `http.post`, etc.) enforce the following limits:

| Limit | Default | Description |
|-------|---------|-------------|
| SSRF protection | on (bypassable) | Blocks requests to localhost, private IPs (`10.x`, `172.16-31.x`, `192.168.x`), and link-local addresses (`169.254.x`). Also rejects non-`http(s)` schemes (e.g. `file://`). Disabled by default in the mock (`AllowLocalNet = true`) since calling other local services is a normal use case; set `AllowLocalNet = false` on the `Runner` to simulate the real API's URL blocking |
| Response body size | 10 MiB | Maximum response body read from an HTTP call |
| Redirect limit | 10 | Maximum number of HTTP redirects followed per request |
| Timeout | 5–180 s (default 60) | Per-request timeout, configurable via the `timeout` call argument |

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/plans` | List plans |
| GET | `/subscriptions` | Get subscription |
| POST | `/subscriptions` | Create subscription |
| DELETE | `/subscriptions` | Delete subscription |
| POST | `/workflows` | Create a workflow |
| GET | `/workflows` | List workflows |
| GET | `/workflows/suggest` | List workflow suggestions |
| GET | `/workflows/{id}` | Get a workflow |
| PATCH | `/workflows/{id}` | Update a workflow |
| DELETE | `/workflows/{id}` | Delete a workflow |
| POST | `/workflows/{id}/revisions` | Create a revision |
| GET | `/workflows/{id}/revisions` | List revisions |
| GET | `/workflows/{id}/revisions/{revisionId}` | Get a revision |
| PUT | `/workflows/{id}/revisions/{revisionId}/revision_alias` | Update revision alias |
| DELETE | `/workflows/{id}/revisions/{revisionId}/revision_alias` | Delete revision alias |
| POST | `/workflows/{id}/executions` | Create an execution |
| GET | `/workflows/{id}/executions` | List executions |
| GET | `/workflows/{id}/executions/{executionId}` | Get an execution |
| POST | `/workflows/{id}/executions/{executionId}/cancel` | Cancel an execution |
| DELETE | `/workflows/{id}/executions/{executionId}` | Delete an execution |
| GET | `/workflows/{id}/executions/{executionId}/exec_history` | List execution history |
