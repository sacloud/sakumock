# sakumock/objectstorage

An Object Storage compatible mock server for local development and testing. It implements the **control plane** of the [SAKURA Cloud Object Storage API](https://github.com/sacloud/sacloud-sdk-go/tree/main/api/object-storage) — sites (clusters), buckets, the per-site account and its access keys, permissions and their access keys, and bucket sub-resources (encryption, replication, plan, usage/quota/penalty) — with in-memory storage.

The **S3-compatible data plane is out of scope**: this mock only models the management API (bucket/account/permission lifecycle) so applications and Terraform can exercise Object Storage CRUD in tests. It does not store objects or speak the S3 protocol.

## Install

```bash
go install github.com/sacloud/sakumock/objectstorage/cmd/sakumock-objectstorage@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock objectstorage` accepts the same flags as `sakumock-objectstorage`.

## Usage

```bash
sakumock-objectstorage
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `OBJECT_STORAGE_LOCALSERVER_ADDR` | `127.0.0.1:18086` | Listen address |
| `--latency` | `OBJECT_STORAGE_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `OBJECT_STORAGE_RATE_LIMIT` | `0` | HTTP rate limit on the API endpoints (requests per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `OBJECT_STORAGE_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `OBJECT_STORAGE_DEBUG` | `false` | Enable debug mode |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/object-storage` client reads the `SAKURA_ENDPOINTS_OBJECT_STORAGE` override and uses it as the API root URL, appending `fed/v1` (federation API) or `<site>/v2` (site API):

```bash
export SAKURA_ENDPOINTS_OBJECT_STORAGE=http://localhost:18086
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

(`sakumock env` emits these for you.)

## Library usage

```go
import "github.com/sacloud/sakumock/objectstorage"

// As http.Handler (for custom servers)
handler, _ := objectstorage.NewHandler(objectstorage.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := objectstorage.NewTestServer(objectstorage.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

The federation API is served under `/fed/v1` and the site (cluster) API under `/{site}/v2`, matching how the SDK's `FedClient` and `SiteClient` join their base paths onto the configured endpoint.

### Federation API (`/fed/v1`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/fed/v1/clusters` | List object storage clusters (sites) |
| GET | `/fed/v1/clusters/{id}` | Get a cluster |
| PUT | `/fed/v1/buckets/{name}` | Create a bucket |
| DELETE | `/fed/v1/buckets/{name}` | Delete a bucket |
| GET/POST/DELETE | `/fed/v1/buckets/{name}/replication` | Read / enable / disable replication |
| GET | `/fed/v1/buckets/{name}/replicable-targets` | List replication target candidates |

### Site API (`/{site}/v2`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/{site}/v2/buckets` | List buckets in the site |
| GET/POST/DELETE | `/{site}/v2/account` | Read / create / delete the site account |
| GET/POST | `/{site}/v2/account/keys` | List / create root access keys |
| GET/DELETE | `/{site}/v2/account/keys/{id}` | Get / delete a root access key |
| GET/POST | `/{site}/v2/permissions` | List / create permissions |
| GET/PUT/DELETE | `/{site}/v2/permissions/{id}` | Get / update / delete a permission |
| GET/POST | `/{site}/v2/permissions/{id}/keys` | List / create a permission's access keys |
| GET/DELETE | `/{site}/v2/permissions/{id}/keys/{key_id}` | Get / delete a permission access key |
| GET | `/{site}/v2/status` | Site status |
| GET | `/{site}/v2/plans` | Bucket plans |
| GET | `/{site}/v2/quota` | Site quota |
| GET | `/{site}/v2/metering/buckets/{name}` | Bucket metering (billing) |
| GET/PUT/DELETE | `/{site}/v2/buckets/{name}/encryption` | Read / enable / disable encryption |
| GET | `/{site}/v2/buckets/{name}/penalty` | Bucket penalty status |
| GET | `/{site}/v2/buckets/{name}/usage` | Bucket usage |
| GET | `/{site}/v2/buckets/{name}/quota` | Bucket quota |
| GET/PUT | `/{site}/v2/buckets/{name}/plan` | Read / change a bucket's plan |

Behavior notes:

- Three static clusters are exposed: `isk01`, `tky01` (standard), and `arc02` (archive). Buckets are federation-global (keyed by name); the per-site `GET /{site}/v2/buckets` returns only the buckets whose `cluster_id` matches the site.
- The per-site account is **not** created automatically (as on the real API, where the control panel creates it on first access). `GET /{site}/v2/account` returns `404` until `POST /{site}/v2/account` is called — the Terraform provider relies on this to create the account on demand.
- Access key and permission key **secrets are only returned when the key is created**; subsequent reads omit the secret, matching the real API.
- Resource IDs (bucket `resource_id`, account `resource_id`, permission `id`) are minted from the shared [`core.IDGenerator`](../core/id.go), so under `sakumock all` they are globally unique across services as on the real platform.
- Bucket deletion is idempotent: deleting a missing bucket still returns `204` (the spec defines no `404` for it), matching Terraform's expectation that deleting an already-gone resource is not an error.
- Bucket usage/quota/penalty return representative placeholder values, and metering returns no billing items, since the mock keeps no usage history.
