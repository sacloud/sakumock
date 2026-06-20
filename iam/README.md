# sakumock/iam

An IAM (Identity and Access Management) compatible mock server for local development and testing. It implements the IAM API with in-memory storage: user management, groups, projects, folders, service principals with OAuth2 token issuance, API keys, IAM/ID roles, organization policies, SSO profiles, and SCIM configurations.

## Install

```bash
go install github.com/sacloud/sakumock/iam/cmd/sakumock-iam@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock iam` accepts the same flags as `sakumock-iam`.

## Usage

```bash
sakumock-iam
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `IAM_LOCALSERVER_ADDR` | `127.0.0.1:18087` | Listen address |
| `--latency` | `IAM_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `IAM_RATE_LIMIT` | `0` | HTTP rate limit shared across all API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `IAM_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--debug` | `IAM_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `IAM_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `IAM_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/iam` client reads the `SAKURA_ENDPOINTS_IAM` override:

```bash
export SAKURA_ENDPOINTS_IAM=http://localhost:18087
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

## Library usage

```go
import "github.com/sacloud/sakumock/iam"

// As http.Handler (for custom servers)
handler, _ := iam.NewHandler(iam.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := iam.NewTestServer(iam.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>
```

## API endpoints

Run `sakumock-iam --routes` for the full list. The server implements the following endpoints:

| Group | Paths |
|-------|-------|
| Users | `GET /compat/users`, `POST /compat/users`, `GET /compat/users/{user_id}`, `PUT /compat/users/{user_id}`, `DELETE /compat/users/{user_id}` |
| User email | `POST /compat/users/{user_id}/register-email`, `POST /compat/users/{user_id}/unregister-email` |
| User OTP | `POST /compat/users/{user_id}/deactivate-otp` |
| Trusted devices | `GET /compat/users/{user_id}/trusted-devices`, `DELETE /compat/users/{user_id}/trusted-devices/{trusted_device_id}`, `POST /compat/users/{user_id}/clear-trusted-devices` |
| Security keys | `GET /compat/users/{user_id}/security-keys`, `PUT /compat/users/{user_id}/security-keys/{security_key_id}`, `DELETE /compat/users/{user_id}/security-keys/{security_key_id}` |
| Groups | `GET /groups`, `POST /groups`, `GET /groups/{group_id}`, `PUT /groups/{group_id}`, `DELETE /groups/{group_id}` |
| Group memberships | `GET /groups/{group_id}/memberships`, `PUT /groups/{group_id}/memberships` |
| Projects | `GET /projects`, `POST /projects`, `GET /projects/{project_id}`, `PUT /projects/{project_id}`, `DELETE /projects/{project_id}`, `POST /move-projects` |
| Project IAM policy | `GET /projects/{project_id}/iam-policy`, `PUT /projects/{project_id}/iam-policy` |
| Folders | `GET /folders`, `POST /folders`, `GET /folders/{folder_id}`, `PUT /folders/{folder_id}`, `DELETE /folders/{folder_id}`, `POST /move-folders` |
| Folder IAM policy | `GET /folders/{folder_id}/iam-policy`, `PUT /folders/{folder_id}/iam-policy` |
| Service principals | `GET /service-principals`, `POST /service-principals`, `GET /service-principals/{service_principal_id}`, `PUT /service-principals/{service_principal_id}`, `DELETE /service-principals/{service_principal_id}` |
| SP keys | `GET /service-principals/{service_principal_id}/keys`, `POST /service-principals/{service_principal_id}/upload-key`, `POST .../keys/{service_principal_key_id}/enable`, `POST .../keys/{service_principal_key_id}/disable`, `DELETE .../keys/{service_principal_key_id}` |
| SP OAuth2 | `POST /service-principals/oauth2/token` |
| API keys | `GET /compat/api-keys`, `POST /compat/api-keys`, `GET /compat/api-keys/{apikey_id}`, `PUT /compat/api-keys/{apikey_id}`, `DELETE /compat/api-keys/{apikey_id}` |
| IAM roles | `GET /iam-roles`, `GET /iam-roles/{iam_role_id}` (read-only) |
| ID roles | `GET /id-roles`, `GET /id-roles/{id_role_id}` (read-only) |
| Organization | `GET /organization`, `PUT /organization` |
| Organization IAM policy | `GET /organization-iam-policy`, `PUT /organization-iam-policy` |
| Organization ID policy | `GET /organization-id-policy`, `PUT /organization-id-policy` |
| Password policy | `GET /organization-password-policy`, `PUT /organization-password-policy` |
| Auth conditions | `GET /organization-auth-conditions`, `PUT /organization-auth-conditions` |
| Auth context | `GET /auth/context` |
| SSO profiles | `GET /sso-profiles`, `POST /sso-profiles`, `GET /sso-profiles/{sso_profile_id}`, `PUT /sso-profiles/{sso_profile_id}`, `DELETE /sso-profiles/{sso_profile_id}`, `POST /sso-profiles/{sso_profile_id}/assign`, `POST /sso-profiles/{sso_profile_id}/unassign` |
| SCIM configurations | `GET /scim-configurations`, `POST /scim-configurations`, `GET /scim-configurations/{id}`, `PUT /scim-configurations/{id}`, `DELETE /scim-configurations/{id}`, `POST /scim-configurations/{id}/regenerate-token` |
| Service policy | `POST /enable-service-policy`, `POST /disable-service-policy`, `GET /service-policy-status`, `GET /organization-service-policy`, `PUT /organization-service-policy`, `GET /service-policy-rule-templates` |

## OpenAPI spec

`openapi/openapi.yaml` is copied from the `github.com/sacloud/sacloud-sdk-go`
module (`api/iam/openapi`). Run `make openapi` to refresh it after
upgrading the SDK dependency.
