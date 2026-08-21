# sakumock/addon

An [Add-on API](https://manual.sakura.ad.jp/api/cloud/portal/openapis/addon-api.json) compatible mock server for local development and testing. It implements every add-on family — AI, CDN, security (DDoS protection / WAF / vulnerability detection), and data analytics (data lake, data warehouse, ETL, query, search, streaming) — with in-memory storage.

The real API deploys an Azure resource group per request, so all families share one lifecycle: list, create (returns the generated resource group and deployment names), get, delete, and — except for vulnerability detection — a deployment status endpoint.

## Install

```bash
go install github.com/sacloud/sakumock/addon/cmd/sakumock-addon@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock addon` accepts the same flags as `sakumock-addon`.

## Usage

```bash
sakumock-addon
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `ADDON_LOCALSERVER_ADDR` | `127.0.0.1:18094` | Listen address |
| `--provisioning-delay` | `ADDON_PROVISIONING_DELAY` | `0` | How long a created resource stays in the `Running` deployment state before list/get can see it (e.g. `5s`); `0` completes immediately |
| `--latency` | `ADDON_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `ADDON_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `ADDON_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--fault` | `ADDON_FAULT` | (none) | Inject faults: `CODE:RATE[:PHASE]`, repeatable (see [Fault Injection](../README.md#fault-injection)) |
| `--debug` | `ADDON_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `ADDON_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `ADDON_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/addon` client reads the `SAKURA_ENDPOINTS_ADDON` override, which replaces the entire API root URL:

```bash
export SAKURA_ENDPOINTS_ADDON=http://localhost:18094
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/addon"

// As http.Handler (for custom servers)
handler, err := addon.NewHandler(addon.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := addon.NewTestServer(addon.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

Every family is served under its own path prefix with the same five operations (four for vulnerability detection, which has no deployment status endpoint).

| Family | Path prefix |
|--------|-------------|
| AI service | `/ai` |
| CDN service | `/cdn` |
| DDoS protection service | `/security/ddos` |
| WAF service | `/security/waf` |
| Vulnerability detection service | `/security/vulnerability` |
| Data lake | `/analytics/datalake` |
| Data warehouse | `/analytics/dwh` |
| Data ETL | `/analytics/etl` |
| Query service | `/analytics/query` |
| Search service | `/analytics/search` |
| Streaming service | `/analytics/streaming` |

| Method | Path | Description |
|--------|------|-------------|
| GET | `<prefix>` | List the family's resource groups |
| POST | `<prefix>` | Create a resource group (`202 Accepted`) |
| GET | `<prefix>/{resourceGroupName}` | Get a resource group |
| DELETE | `<prefix>/{resourceGroupName}` | Delete a resource group (`204 No Content`) |
| GET | `<prefix>/status/{resourceGroupName}/{deploymentName}` | Get the deployment status (not available for `/security/vulnerability`) |

Run `sakumock addon --routes` for the fully expanded table.

## Mock behavior

- **Generated names.** Create returns a generated `resourceGroupName` (`<family>-<8 hex digits>`) and `deploymentName` (`<family>-deployment-<8 hex digits>`), which are the handles for every later call. Resource groups are per family: one family's name is invisible to another's endpoints.
- **Resource data.** `GetResourceResponse.data` is free-form in the spec: it carries the deployed Azure resource, whose shape differs per family. The mock rebuilds that resource from the create request it stored, so a client — the `sacloud/sakura` Terraform provider does exactly this — can recover every setting it sent. See [Resource data](#resource-data) for the fields. The portal `url` is a synthetic `https://secure.sakura.ad.jp/cloud/addon/...` link the mock composes.
- **Provisioning.** With `--provisioning-delay` set, a created resource group reports `provisioningState: "Running"` from the status endpoint and stays invisible to list/get (which is how the real API behaves while a deployment is running) until the delay elapses, then flips to `Succeeded`. The default is to complete immediately.
- **Install script.** `POST /security/vulnerability` returns an `installScript` as the real API does, but the script is a harmless stand-in (Windows or Linux flavored by the request's `os`) that only prints that nothing was installed.
- **Validation.** Request bodies are checked against the constraints generated from the OpenAPI spec into `validate_gen.go` (required fields, enums, lengths, ranges). Rejections are `400` with the spec's `{"errors": [{"code", "message"}]}` envelope.

### Resource data

`data` is free-form in the spec, so what a client can rely on is what the real API puts there: the deployed Azure resource. The fields below are the ones the [`sacloud/sakura`](https://registry.terraform.io/providers/sacloud/sakura/latest) Terraform provider decodes to recover a resource's settings, and the mock reproduces them from the create request:

| Family | Fields in `data` |
|--------|------------------|
| AI service | `location.name` (an object, unlike the other families), `sku.name` |
| CDN / WAF / DDoS protection | `location`, `sku.name` (`<Level>_AzureFrontDoor`), `endpoints[0].routes[0].properties.patternsToMatch`, `originGroups[0].origins[0].properties.{hostName,originHostHeader}` |
| Data lake | `location`, `sku.name` (`<Performance>_<Redundancy>`, e.g. `Standard_LRS`) |
| Search service | `location`, `sku.name` (`free` / `basic` / `standard1` …), `properties.{partitionCount,replicaCount,hostingMode}` |
| Streaming service | `location`, `sku.capacity` (the streaming unit count) |
| Data warehouse / ETL / query / vulnerability detection | `location` |

Every family also reports `name` (the resource group name) and `properties.provisioningState`.

```console
$ curl localhost:18094/ai/ai-c00e8994
{
  "data": {
    "name": "ai-c00e8994",
    "location": { "name": "japaneast", "displayName": "Japan East" },
    "sku": { "name": "S0" },
    "properties": { "provisioningState": "Succeeded" }
  },
  "url": "https://secure.sakura.ad.jp/cloud/addon/ai/ai-c00e8994"
}

$ curl localhost:18094/cdn/cdn-cb726c22
{
  "data": {
    "name": "cdn-cb726c22",
    "location": "japaneast",
    "sku": { "name": "Standard_AzureFrontDoor" },
    "properties": { "provisioningState": "Succeeded" },
    "endpoints": [
      {
        "name": "cdn-cb726c22-endpoint",
        "routes": [
          {
            "name": "cdn-cb726c22-route",
            "properties": {
              "patternsToMatch": ["/*"],
              "supportedProtocols": ["Http", "Https"]
            }
          }
        ]
      }
    ],
    "originGroups": [
      {
        "name": "cdn-cb726c22-origin-group",
        "origins": [
          {
            "name": "cdn-cb726c22-origin",
            "properties": {
              "hostName": "origin.example.com",
              "originHostHeader": "cdn.example.com"
            }
          }
        ]
      }
    ]
  },
  "url": "https://secure.sakura.ad.jp/cloud/addon/cdn/cdn-cb726c22"
}
```

The list endpoint is unaffected: its items are the spec's typed `ResourceGroupResource` (`id`, `hasData`, `data` as `ResourceGroupData`, `url`).

## Use with Terraform

The [`sacloud/sakura`](https://registry.terraform.io/providers/sacloud/sakura/latest) provider's `sakura_addon_*` resources work against the mock — `terraform apply`, a no-diff `plan`, and `destroy` for all ten of them are covered by the repository's [end-to-end test](../test/terraform/addon.tf). Point the provider at the mock with `sakumock env`, which emits `SAKURA_ENDPOINTS_ADDON` along with every other service's endpoint.

## Mock-only endpoints

Endpoints under `/_sakumock/` do not exist in the real SAKURA Cloud API; they observe the mock itself.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/_sakumock/spec-violations` | List responses that diverged from the OpenAPI spec |
| `DELETE` | `/_sakumock/spec-violations` | Clear the recorded violations |

Every handler response is validated against the OpenAPI spec (status code declared, JSON body matching the response schema). A violation never alters the response the client receives: it is logged at `WARN` level and recorded — deduplicated with a count — for inspection. An empty list after exercising the endpoints you care about means the mock's responses conform to the spec.
