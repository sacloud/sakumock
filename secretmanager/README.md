# sakumock/secretmanager

A SecretManager-compatible mock server for local development and testing. It implements the secret API (list, create, delete, unveil) with in-memory storage and secret versioning.

## Install

```bash
go install github.com/sacloud/sakumock/secretmanager/cmd/sakumock-secretmanager@latest
```

## Usage

```bash
sakumock-secretmanager
# listening on 127.0.0.1:18082
```

### Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--addr` | `SECRETMANAGER_LOCALSERVER_ADDR` | `127.0.0.1:18082` | Listen address |
| `--latency` | `SECRETMANAGER_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `SECRETMANAGER_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `SECRETMANAGER_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `SECRETMANAGER_DEBUG` | `false` | Enable debug mode |

```bash
# Add 500ms latency to every response (useful for timeout testing)
sakumock-secretmanager --latency 500ms

# Mimic the production quota of 100 requests per minute
sakumock-secretmanager --rate-limit 100 --rate-limit-window 1m
```

## Use with secretmanager-api-go SDK / sakura-secrets-cli

```bash
export SAKURA_ENDPOINTS_SECRETMANAGER=http://localhost:18082
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
export VAULT_ID=your-vault-id

sakura-secrets-cli list
sakura-secrets-cli get my-secret
```

Vaults are created automatically on first access. All data is stored in memory and lost when the server stops.

## Use as a library

```go
import "github.com/sacloud/sakumock/secretmanager"

// As http.Handler (for embedding in your own server)
handler := secretmanager.NewHandler(secretmanager.Config{})
defer handler.Close()

// As a test server (for use in tests)
srv := secretmanager.NewTestServer(secretmanager.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/secretmanager/vaults/{vault_id}/secrets` | List secrets |
| `POST` | `/secretmanager/vaults/{vault_id}/secrets` | Create or update a secret |
| `DELETE` | `/secretmanager/vaults/{vault_id}/secrets` | Delete a secret |
| `POST` | `/secretmanager/vaults/{vault_id}/secrets/unveil` | Retrieve a secret value |
