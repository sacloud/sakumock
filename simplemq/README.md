# sakumock/simplemq

A SimpleMQ-compatible mock server for local development and testing. It implements the message API (send, receive, delete, extend timeout) with in-memory or SQLite-backed persistent storage.

## Install

```bash
go install github.com/sacloud/sakumock/simplemq/cmd/sakumock-simplemq@latest
```

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
| `--debug` | `SIMPLEMQ_DEBUG` | `false` | Enable debug mode |

```bash
# Require a specific API key
sakumock-simplemq --api-key my-secret-key

# Use SQLite for persistent storage
sakumock-simplemq --database ./messages.db

# Add 500ms latency to every response (useful for timeout testing)
sakumock-simplemq --latency 500ms
```

## Use with simplemq-api-go SDK or simplemq-cli

```bash
export SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://localhost:18080
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy

simplemq-cli message send --queue myqueue "Hello!"
simplemq-cli message receive --queue myqueue
```

Queues are created automatically on first access. When using in-memory storage (default), all data is lost when the server stops. Use `--database` for persistent storage.

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

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/queues/{queueName}/messages` | Send a message |
| `GET` | `/v1/queues/{queueName}/messages` | Receive a message |
| `PUT` | `/v1/queues/{queueName}/messages/{messageID}` | Extend visibility timeout |
| `DELETE` | `/v1/queues/{queueName}/messages/{messageID}` | Delete a message |

All endpoints require `Authorization: Bearer <token>` header.

## Storage Backends

- **Memory** (default): Fast, data lost on restart.
- **SQLite** (`--database` option): Persistent, WAL mode, pure Go (no CGO required).
