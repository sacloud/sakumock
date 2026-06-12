# sakumock/simplemq

A SimpleMQ-compatible mock server for local development and testing. It implements both the data plane message API (send, receive, delete, extend timeout) and the control plane resource API (create/list/get/update/delete queues, message count, API key rotation, clearing a queue), with in-memory or SQLite-backed persistent storage.

## Install

```bash
go install github.com/sacloud/sakumock/simplemq/cmd/sakumock-simplemq@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock simplemq` accepts the same flags as `sakumock-simplemq`.

## Usage

```bash
sakumock-simplemq
# listening on 127.0.0.1:18080
```

### Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--addr` | `SIMPLEMQ_LOCALSERVER_ADDR` | `127.0.0.1:18080` | Listen address |
| `--api-key` | `SIMPLEMQ_API_KEY` | (empty) | API key for authentication (if empty, any non-empty key is accepted) |
| `--visibility-timeout` | `SIMPLEMQ_VISIBILITY_TIMEOUT` | `30s` | Visibility timeout |
| `--message-expire` | `SIMPLEMQ_MESSAGE_EXPIRE` | `96h` | Message expire time (default: 4 days) |
| `--database` | `SIMPLEMQ_DATABASE` | (empty) | SQLite database path for persistent storage |
| `--latency` | `SIMPLEMQ_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `SIMPLEMQ_RATE_LIMIT` | `0` | Per-queue HTTP rate limit (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `SIMPLEMQ_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--strict` | `SIMPLEMQ_STRICT` | `false` | Strict mode: the data plane only accepts queues created via the control plane, authenticated with the queue's issued API key (see [Strict mode](#strict-mode)) |
| `--debug` | `SIMPLEMQ_DEBUG` | `false` | Enable debug mode |

```bash
# Require a specific API key
sakumock-simplemq --api-key my-secret-key

# Use SQLite for persistent storage
sakumock-simplemq --database ./messages.db

# Add 500ms latency to every response (useful for timeout testing)
sakumock-simplemq --latency 500ms

# Rate limit each queue to 10 req/sec (excess returns 429 + Retry-After)
sakumock-simplemq --rate-limit 10

# Or 100 req/min, matching production-like quotas
sakumock-simplemq --rate-limit 100 --rate-limit-window 1m
```

## Use with sacloud-sdk-go or simplemq-cli

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/simplemq` client and
simplemq-cli read the endpoint overrides below. Both the data plane (message) and control
plane (queue) APIs are served by the same mock process, so point both overrides at it:

```bash
export SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://localhost:18080
export SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE=http://localhost:18080
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy

# Data plane: send/receive messages
simplemq-cli message send --queue myqueue "Hello!"
simplemq-cli message receive --queue myqueue
```

Queue resources (control plane) can be managed with the SDK's queue client or any
tool that talks to the `/commonserviceitem` endpoints.

A queue created via the control plane uses the visibility timeout and expiration set
on it (`configQueue`). For data plane requests to a queue that was never created via
the control plane, the queue is created automatically on first access using the
server's default settings. When using in-memory storage (default), all data is lost
when the server stops. Use `--database` for persistent storage.

## Use as a library

```go
import "github.com/sacloud/sakumock/simplemq"

// As http.Handler (for embedding in your own server)
handler, err := simplemq.NewHandler(simplemq.Config{
    VisibilityTimeout: 30 * time.Second,
    MessageExpire:     96 * time.Hour,
})
defer handler.Close()

// As a test server (for use in tests)
srv := simplemq.NewTestServer(simplemq.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API Endpoints

### Data plane (message API)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/queues/{queueName}/messages` | Send a message |
| `GET` | `/v1/queues/{queueName}/messages` | Receive a message |
| `PUT` | `/v1/queues/{queueName}/messages/{messageID}` | Extend visibility timeout |
| `DELETE` | `/v1/queues/{queueName}/messages/{messageID}` | Delete a message |

Data plane endpoints require an `Authorization: Bearer <token>` header. When
`--api-key` is set the token must match it; otherwise any non-empty token is accepted.

### Control plane (queue resource API)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/commonserviceitem` | Create a queue |
| `GET` | `/commonserviceitem` | List queues |
| `GET` | `/commonserviceitem/{id}` | Get a queue |
| `PUT` | `/commonserviceitem/{id}` | Update queue settings |
| `DELETE` | `/commonserviceitem/{id}` | Delete a queue |
| `GET` | `/commonserviceitem/{id}/simplemq/message-count` | Get message count |
| `PUT` | `/commonserviceitem/{id}/simplemq/rotate-apikey` | Rotate the queue API key |
| `DELETE` | `/commonserviceitem/{id}/simplemq/messages` | Clear all messages in a queue |

Control plane endpoints use HTTP Basic authentication (the SAKURA Cloud access
token / secret). The mock accepts any non-empty credentials and does not validate
them against `--api-key`.

## Strict mode

By default the data plane is permissive: queues are created automatically on first
access and any non-empty Bearer token (or `--api-key`, if set) is accepted. This is
convenient for quick local testing.

Pass `--strict` to model the real SAKURA Cloud flow instead. In strict mode the data
plane:

1. Only serves queues that were created via the control plane (`POST /commonserviceitem`); requests to an unknown queue are rejected with `401`.
2. Authenticates each request with the queue's own API key — the one returned by `rotate-apikey`. A queue has no usable key until `rotate-apikey` is called, and rotating again invalidates the previous key.

`--strict` and `--api-key` are mutually exclusive: the data plane is authenticated
either by a single shared key (`--api-key`) or by per-queue issued keys (`--strict`),
not both. Passing both is a startup error.

The real-world flow against a strict server is:

1. Create the queue via the control plane (`POST /commonserviceitem`).
2. Issue the data plane key via `rotate-apikey` (`PUT /commonserviceitem/{id}/simplemq/rotate-apikey`) and keep the returned `apikey`.
3. Use that `apikey` as the `Authorization: Bearer <apikey>` token for the message API.

## Storage Backends

- **Memory** (default): Fast, data lost on restart.
- **SQLite** (`--database` option): Persistent, WAL mode, pure Go (no CGO required).
