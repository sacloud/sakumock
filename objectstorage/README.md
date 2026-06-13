# sakumock/objectstorage

An Object Storage compatible mock server for local development and testing. It implements the **control plane** of the [SAKURA Cloud Object Storage API](https://github.com/sacloud/sacloud-sdk-go/tree/main/api/object-storage) — sites (clusters), buckets, the per-site account and its access keys, permissions and their access keys, and bucket sub-resources (encryption, replication, plan, usage/quota/penalty) — with in-memory storage.

The control plane does not store objects or speak the S3 protocol itself. An **optional S3-compatible data plane** can be enabled (`--enable-data-plane`), which sakumock serves by launching an external [versitygw](https://github.com/versity/versitygw) process — see [Data plane (S3)](#data-plane-s3) below.

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
| `--enable-data-plane` | `OBJECT_STORAGE_ENABLE_DATA_PLANE` | `false` | Serve the S3-compatible data plane via an external versitygw process |
| `--data-plane-addr` | `OBJECT_STORAGE_DATA_PLANE_ADDR` | `127.0.0.1:28086` | Listen address for the S3 data plane (control-plane port + 10000) |
| `--data-plane-dir` | `OBJECT_STORAGE_DATA_PLANE_DIR` | (temp dir) | Backend directory; empty uses a temp dir removed on shutdown |
| `--data-plane-region` | `OBJECT_STORAGE_DATA_PLANE_REGION` | `jp-north-1` | Region the S3 data plane signs/validates requests for |

The data plane's root credentials are fixed development defaults (access key `sakumock`, secret key `sakumocksecret`) — not configurable, like the dummy SAKURA credentials.

(Under the unified binary these are prefixed, e.g. `--objectstorage-enable-data-plane`.)

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

## Data plane (S3)

The S3-compatible object API (PUT/GET/DELETE objects, multipart, …) is **not** reimplemented. Instead, with `--enable-data-plane`, sakumock launches an external [versitygw](https://github.com/versity/versitygw) process backed by a local POSIX directory and manages its lifecycle (start on serve, graceful stop on shutdown). versitygw is **not bundled** — it would bloat the released single binary and the distroless image — so it must be installed on `PATH`. Because `--enable-data-plane` is an explicit opt-in, sakumock **fails to start** if versitygw is not found or does not come up, rather than silently running without the data plane you asked for.

```bash
# install versitygw (https://github.com/versity/versitygw), then:
sakumock objectstorage --enable-data-plane
# or under the unified binary:
sakumock all --objectstorage-enable-data-plane
```

The integration is **loose**:

- **Bucket existence is mirrored**: creating/deleting a bucket through the control plane creates/removes a directory in the versitygw backend, which versitygw exposes as an S3 bucket. Objects themselves live only in versitygw.
- **A single fixed root credential** (access key `sakumock`, secret key `sakumocksecret`) authenticates S3 requests. Control-plane access keys and permissions are **not** enforced on the data plane.

When the data plane is enabled, the startup log and `sakumock env` emit the `AWS_*` variables an aws-cli / aws-sdk client needs (`AWS_ENDPOINT_URL_S3`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`). Load them and use any S3 client without passing `--endpoint-url`:

```bash
# `env` is a subcommand of the unified binary, so the flag is prefixed:
sakumock env --objectstorage-enable-data-plane > sakumock.env
set -a; source sakumock.env; set +a

aws s3 ls
aws s3 cp ./file.txt s3://my-bucket/file.txt
```

(`AWS_ENDPOINT_URL_S3` and `AWS_DEFAULT_REGION` are honored by both aws-cli and aws-sdk-go-v2.)

Note: the SAKURA Terraform provider's `sakura_object_storage_object` resource uses an S3 client with TLS forced on, so reaching this plain-HTTP data plane from that specific resource needs versitygw served over TLS; SDK/CLI/application clients (where you control the endpoint and TLS) work over HTTP.
