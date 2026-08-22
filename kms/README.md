# sakumock/kms

A KMS (Key Management Service) compatible mock server for local development and testing. It implements the key management API (CRUD, rotate, status change, schedule destruction, encrypt, decrypt) with in-memory storage.

## Install

```bash
go install github.com/sacloud/sakumock/kms/cmd/sakumock-kms@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock kms` accepts the same flags as `sakumock-kms`.

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
| `--key` | `KMS_KEY` | (none) | Pre-create a key with a fixed ID and key material: `ID=SECRET`, repeatable (see [Fixed keys](#fixed-keys)) |
| `--fault` | `KMS_FAULT` | (none) | Inject faults: `CODE:RATE[:PHASE]`, repeatable (see [Fault Injection](../README.md#fault-injection)) |
| `--debug` | `KMS_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `KMS_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `KMS_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

### Fixed keys

Keys created through the API get a fresh random ID and random AES-256 key material, so a ciphertext cannot be decrypted after the mock restarts. For local development where an encrypted value (e.g. a wrapped data encryption key) is kept in a config file, `--key ID=SECRET` pre-creates a key at startup with a fixed ID and key material derived from `SECRET`, so the same ciphertext decrypts across restarts:

```bash
sakumock kms --key 123456789012=my-dev-secret
# or, under the unified binary:
sakumock all --kms-key 123456789012=my-dev-secret
```

- `ID` is the numeric resource ID (at most 12 digits) the key is served under (`/keys/123456789012/...`). Generated IDs never collide with it, and under `sakumock all` an ID already fixed by another service is a startup error.
- `SECRET` is either 64 hex characters, used verbatim as the 32-byte AES-256 key, or any other string, whose SHA-256 digest becomes the key material. Changing `SECRET` changes the material, so existing ciphertexts stop decrypting.
- The flag is a map: repeat it (`--key A=... --key B=...`) or separate entries with `;` (`--key A=...;B=...`). The `KMS_KEY` environment variable takes the same `ID=SECRET[;ID=SECRET]` form. In a `sakumock all --config` file it is a mapping: `kms: {key: {"123456789012": "my-dev-secret"}}`.
- Preset keys behave like any other key (listed, rotatable, deletable); a rotation's new version is random, so only version 1 is stable across restarts.

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/kms` client reads the `SAKURA_ENDPOINTS_KMS` override:

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

## Mock-only endpoints

Endpoints under `/_sakumock/` do not exist in the real SAKURA Cloud API; they observe the mock itself.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/_sakumock/spec-violations` | List responses that diverged from the OpenAPI spec |
| `DELETE` | `/_sakumock/spec-violations` | Clear the recorded violations |

Every handler response is validated against the OpenAPI spec (status code declared, JSON body matching the response schema). A violation never alters the response the client receives: it is logged at `WARN` level and recorded — deduplicated with a count — for inspection. An empty list after exercising the endpoints you care about means the mock's responses conform to the spec.
