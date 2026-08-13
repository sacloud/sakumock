# sakumock/apigw

An API Gateway compatible mock server for local development and testing. It implements the gateway management API (services, routes, route authorization/transformations, users, groups, domains, certificates, plans, subscriptions, OIDC configurations) with in-memory storage.

Each service gets an auto-issued gateway hostname (`routeHost`) of the form `site-<id>.localhost`. With `--enable-data-plane`, the gateway itself runs on a separate listener (default `127.0.0.1:28091`): requests are matched by Host header, path, and method against the configured routes, authenticated per the service's `authentication` scheme, and reverse proxied to the owning service's upstream.

## Install

```bash
go install github.com/sacloud/sakumock/cmd/sakumock-apigw@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock apigw` accepts the same flags as `sakumock-apigw`.

## Usage

```bash
sakumock-apigw
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `APIGW_LOCALSERVER_ADDR` | `127.0.0.1:18091` | Listen address |
| `--latency` | `APIGW_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `APIGW_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `APIGW_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--fault` | `APIGW_FAULT` | (none) | Inject faults: `CODE:RATE[:PHASE]`, repeatable (see [Fault Injection](../README.md#fault-injection)) |
| `--enable-data-plane` | `APIGW_ENABLE_DATA_PLANE` | `false` | Enable the gateway data plane: a separate listener routing requests by Host header to the configured upstreams |
| `--data-plane-addr` | `APIGW_DATA_PLANE_ADDR` | `127.0.0.1:28091` | Data plane listen address (control-plane port + 10000) |
| `--debug` | `APIGW_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `APIGW_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `APIGW_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/apigw` client reads the `SAKURA_ENDPOINTS_APIGW` override:

```bash
export SAKURA_ENDPOINTS_APIGW=http://localhost:18091
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/apigw"

// As http.Handler (for custom servers)
handler, err := apigw.NewHandler(apigw.Config{})
if err != nil {
    panic(err)
}
defer handler.Close()

// As test server (for integration tests)
srv := apigw.NewTestServer(apigw.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## Data plane

With `--enable-data-plane`, the gateway listens on `--data-plane-addr` and serves the configured routes:

```bash
sakumock-apigw --enable-data-plane

# Create a subscription, a service (upstream), and a route via the management
# API, note the service's routeHost (site-<id>.localhost), then:
curl http://site-0123456789ab.localhost:28091/your/path
```

`*.localhost` resolves to loopback on typical systems, so the auto-issued hostname works without DNS setup. Custom domains are reached by setting the Host header explicitly:

```bash
curl -H "Host: api.example.com" http://127.0.0.1:28091/your/path
```

Matching and forwarding semantics (Kong-style, per the spec):

- A request matches a route when its Host is in the route's effective host set (`hosts`, or the auto-issued `host` when `hosts` is empty), its path matches the route `path`, and its method is in `methods`.
- A route `path` starting with `~/` is a regular expression anchored at the start; anything else is a literal prefix. Regex routes rank above prefix routes; among regex routes the lowest `regexPriority` wins (0 is highest), among prefix routes the longest prefix wins.
- `stripPath` (default true) removes the matched portion before forwarding; the service `path` is prepended. `preserveHost` (default false) keeps the client's Host header instead of the upstream host.
- An http request matching an https-only route gets the `httpsRedirectStatusCode` response (3xx redirect, or the default 426 upgrade).
- No match returns `404 {"message":"no Route matched with those values"}`; upstream failures return 502 and timeouts 504 — the same messages as the real gateway. `connectTimeout` bounds the dial; `readTimeout`/`writeTimeout` bound the idle time between successive read/write operations on the upstream connection (nginx-style semantics, as in the real gateway).
- **Object storage backend**: a service with `objectStorageConfig` serves objects from the configured S3-compatible endpoint (SigV4, path-style) instead of proxying — any endpoint works: sakumock's own objectstorage data plane, MinIO, or real object storage. When targeting sakumock's objectstorage data plane, set `region` to its `--data-plane-region` (default `jp-north-1`) — versitygw validates the SigV4 credential scope, so a mismatch is a 403. The (stripPath-applied) request path plus `folderName` is the object key; `useDocumentIndex` (default) appends `index.html` to directory paths. Routes of such services allow only GET, HEAD, and OPTIONS (per the spec); a missing object is 404, backend failures are 502.
- **CORS** (`corsConfig` on a service): preflights (OPTIONS + `Origin` + `Access-Control-Request-Method`) are answered by the gateway **before authentication** (browsers send them without credentials), honoring `accessControlAllowOrigins` (comma-separated or `*`), `accessControlAllowMethods`/`Headers`, `maxAge`, `credentials` (a wildcard origin is echoed back when credentials are on), `privateNetwork`, and `preflightContinue` (pass preflights to the upstream; for object-storage services the preflight is forwarded to the S3 endpoint, whose bucket CORS configuration answers it). Actual responses gain `Access-Control-Allow-Origin`/`-Credentials`/`-Expose-Headers` and `Vary: Origin`. The route must match the OPTIONS method for preflights, so include `OPTIONS` in `methods`.
- **Request/response transformations** configured on a route are applied when proxying, with the real gateway's semantics: phases run allow → remove → rename → replace → add → append; `replace` only touches existing keys, `add` only missing ones, `append` accumulates; response operations respect `ifStatusCode`. Body operations apply to JSON object bodies only (dotted keys address nested fields); non-JSON and content-encoded bodies pass through untouched.
- `retries` is best-effort: only connection-level failures of bodyless requests are retried. `X-Forwarded-For/-Proto/-Host` are set on forwarded requests. Upstream TLS certificates are **not verified** (mock convenience for self-signed local upstreams).
- With the common `--tls-cert`/`--tls-key`, the data plane serves HTTPS and routes match the `https` protocol.

Authentication (the service's `authentication` field) is enforced before proxying:

- **basic**: standard Basic authentication against the user's `basicAuth` credential.
- **hmac**: draft-cavage style signatures (`Authorization: hmac username="...",algorithm="hmac-sha256",headers="date request-line",signature="..."`), algorithms `hmac-sha1/256/384/512`, with a 300s clock skew check on the `Date`/`X-Date` header when it is part of the signature. Request-body digests are not validated (matching the real gateway's default).
- **jwt**: `Authorization: Bearer` tokens; the credential is resolved by the token's `iss` claim matching the credential `key`, and the signature is verified with the credential's algorithm (HS256/384/512) and secret. `exp`/`nbf` are validated.
- **oidc**: Bearer token validation (the `accessToken` authentication method). The token is verified against the configured `issuer` via OpenID Connect discovery and JWKS — any IdP works, including real ones (e.g. `https://accounts.google.com`; send a Google **ID token**, not the opaque access token). The token's `aud` must contain the `clientId`, or — when `tokenAudiences` is set — one of its values. `hideCredentials` strips `Authorization` before proxying. OIDC consumers are external to the user store, so group ACLs and user-level IP restrictions do not apply to them.
- **oidc (authorizationCodeFlow)**: browser login. An unauthenticated request is redirected to the IdP's authorization endpoint with the protected URL itself as `redirect_uri`; the callback exchanges the code, verifies the ID token (including the nonce), sets the gateway session cookie (`apigw_session`, HttpOnly, stripped before proxying), and redirects back. Sessions live in memory until the ID token expiry. A session is always established on a completed login, regardless of `useSession` (the flow cannot work without one).
  - **Redirect URI registration at the IdP**: register the gateway URL(s) you protect, e.g. `https://site-<id>.localhost:28091/...` when running with the common `--tls-cert`/`--tls-key` (Google requires https except plain `http://localhost`; a locally-trusted certificate via mkcert works well). On plain http, registering a custom `Domain` named `localhost` and putting it in the route's `hosts` lets you use `http://localhost:28091/...`.
- Route **authorization** (group ACL): when enabled, the authenticated user must belong to one of the enabled groups, otherwise 403.
- **IP restrictions**: the route-level restriction applies before authentication; the user-level restriction applies after authentication and only when the route has none configured (per the API reference).
- Credential identifiers (`basicAuth`/`hmacAuth` `userName`, `jwt` `key`) must be unique across users because the data plane resolves the consumer by them; reusing one is rejected with 400.

## Behavior notes

- **Plans** are seeded at startup (`トライアル` and `エンタープライズ`, mirroring the real service's public pricing) and read-only; plan IDs are deterministic across restarts.
- **Subscriptions** bind to at most one service, as in the real API. `POST /subscriptions` returns 204 with no body; look the subscription up via `GET /subscriptions` (this matches the real API and is what the SDK/Terraform provider do).
- **routeHost** is `site-<first 12 hex of the service id>.localhost`. `*.localhost` resolves to loopback on typical systems, so gateway hostnames need no DNS setup once the data plane exists.
- **Route `hosts`** entries must be the service's `routeHost` or a registered domain's `domainName`, per the spec.
- **Referential integrity**: deleting a service with routes, a domain referenced by a route's `hosts`, a certificate referenced by a domain, or a subscription bound to a service is refused (400); deleting an OIDC configuration referenced by a service is refused (409). Deleting a group cascades out of user memberships and route authorizations.
- **Secrets are echoed**: credential values (`password`, `secret`, OIDC `clientSecret`, object storage keys) are returned by GET endpoints because the SDK's generated client requires those fields on decode. Do not put real secrets into the mock.
- **Certificates**: `expiredAt` is derived by parsing the uploaded PEM; the PEM material itself is never returned (writeOnly in the spec).

## Mock-only endpoints

Endpoints under `/_sakumock/` do not exist in the real SAKURA Cloud API; they observe the mock itself.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/_sakumock/spec-violations` | List responses that diverged from the OpenAPI spec |
| `DELETE` | `/_sakumock/spec-violations` | Clear the recorded violations |

Every handler response is validated against the OpenAPI spec (status code declared, JSON body matching the response schema). A violation never alters the response the client receives: it is logged at `WARN` level and recorded — deduplicated with a count — for inspection. An empty list after exercising the endpoints you care about means the mock's responses conform to the spec.
