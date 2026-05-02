# sakumock/kms

A KMS (Key Management Service) compatible mock server for local development and testing. It implements the key management API (CRUD, rotate, status change, schedule destruction, encrypt, decrypt) with in-memory storage.

## Install

```bash
go install github.com/sacloud/sakumock/kms/cmd/sakumock-kms@latest
```

## Usage

```bash
sakumock-kms
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `KMS_LOCALSERVER_ADDR` | `127.0.0.1:18081` | Listen address |
| `--latency` | `KMS_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `KMS_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `KMS_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `KMS_DEBUG` | `false` | Enable debug mode |

## Use with kms-api-go SDK

```bash
export SAKURA_ENDPOINTS_KMS=http://localhost:18081
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/kms"

// As http.Handler (for custom servers)
handler := kms.NewHandler(kms.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := kms.NewTestServer(kms.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/kms/keys` | List all keys |
| POST | `/kms/keys` | Create a new key |
| GET | `/kms/keys/{resource_id}` | Get a key |
| PUT | `/kms/keys/{resource_id}` | Update a key |
| DELETE | `/kms/keys/{resource_id}` | Delete a key |
| POST | `/kms/keys/{resource_id}/rotate` | Rotate a key |
| POST | `/kms/keys/{resource_id}/status` | Change key status |
| POST | `/kms/keys/{resource_id}/schedule-destruction` | Schedule key destruction |
| POST | `/kms/keys/{resource_id}/encrypt` | Encrypt data |
| POST | `/kms/keys/{resource_id}/decrypt` | Decrypt data |
