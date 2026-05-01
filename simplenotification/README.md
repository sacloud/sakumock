# sakumock/simplenotification

A Simple Notification compatible mock server for local development and testing. It implements only the message-sending endpoint of the [SAKURA Cloud Simple Notification API](https://github.com/sacloud/simple-notification-api-go) with in-memory storage. Destination groups, routing, and history APIs are out of scope — this mock focuses on letting applications exercise notification dispatch in tests.

## Install

```bash
go install github.com/sacloud/sakumock/simplenotification/cmd/sakumock-simplenotification@latest
```

## Usage

```bash
sakumock-simplenotification
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `SIMPLENOTIFICATION_LOCALSERVER_ADDR` | `127.0.0.1:18083` | Listen address |
| `--latency` | `SIMPLENOTIFICATION_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--exec` | `SIMPLENOTIFICATION_EXEC` | (none) | Shell command run for each accepted message (see [Per-message exec hook](#per-message-exec-hook)) |
| `--debug` | `SIMPLENOTIFICATION_DEBUG` | `false` | Enable debug mode |

### Per-message exec hook

When `--exec` is set, the mock spawns the given shell command for every accepted message:

- The command is invoked as `sh -c '<command>'`, so shell features (pipes, env interpolation, redirects) are available. Windows is not supported.
- The message body is piped to the command's stdin.
- The command's stdout and stderr are inherited from the mock process, so any output appears in the same terminal as the server logs.
- Metadata is exposed as environment variables: `SAKUMOCK_GROUP_ID`, `SAKUMOCK_MESSAGE_ID`, `SAKUMOCK_CREATED_AT`.
- The command is run asynchronously (fire-and-forget). The HTTP response is always `202` regardless of whether the command succeeds; failures are logged at warn level.

Examples:

```bash
# Print every notification to the terminal
sakumock-simplenotification --exec 'cat; echo'

# Show as a desktop notification (Linux)
sakumock-simplenotification --exec 'notify-send "sakura notification ($SAKUMOCK_GROUP_ID)" "$(cat)"'

# Forward to a personal Slack webhook during local dev
sakumock-simplenotification --exec 'jq -Rs "{text:.}" | curl -sS -X POST -H "content-type: application/json" -d @- "$SLACK_WEBHOOK"'
```

## Use with simple-notification-api-go SDK

```bash
export SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=http://localhost:18083
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/simplenotification"

// As http.Handler (for custom servers)
handler := simplenotification.NewHandler(simplenotification.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := simplenotification.NewTestServer(simplenotification.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>

// Inspect messages accepted by the mock
for _, m := range srv.Messages() {
    fmt.Println(m.GroupID, m.Message)
}
```

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/commonserviceitem/{id}/simplenotification/message` | Send a notification message to the specified group (`id` must be a 12-digit number) |

The handler accepts any 12-digit numeric `id`; group existence is not checked. Messages are validated to be non-empty and at most 2048 characters long. On success the server responds with `202 Accepted` and `{"is_ok": true}`.

## Inspection endpoints

These endpoints are sakumock-specific (not part of the SAKURA Cloud API) and let test code in any language verify what the application sent:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/_sakumock/messages` | Return all accepted messages in send order |
| DELETE | `/_sakumock/messages` | Clear all accepted messages |

`GET` response shape:

```json
{
  "messages": [
    {
      "id": "1",
      "group_id": "123456789012",
      "message": "hello",
      "created_at": "2026-05-01T12:34:56.789Z"
    }
  ]
}
```

Use `DELETE` between test cases to reset state when sharing a single server across tests.
