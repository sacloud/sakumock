# Changelog

## [v0.9.1](https://github.com/sacloud/sakumock/compare/v0.9.0...v0.9.1) - 2026-08-25
### Other Changes
- kms: deterministic key rotation and --key ID=SECRET@N by @fujiwara in https://github.com/sacloud/sakumock/pull/173

## [v0.9.0](https://github.com/sacloud/sakumock/compare/v0.8.0...v0.9.0) - 2026-08-22
### 🚀 New Features
- Add seg (Service Endpoint Gateway) mock service by @tokuhirom in https://github.com/sacloud/sakumock/pull/166
- addon: add the Add-on API mock service by @tokuhirom in https://github.com/sacloud/sakumock/pull/167
- kms: add --key to pre-create keys with fixed ID and key material by @fujiwara in https://github.com/sacloud/sakumock/pull/171
### 📦 Dependency Updates
- Bump sacloud-sdk-go to v0.1.0 by @fujiwara in https://github.com/sacloud/sakumock/pull/170
### Other Changes
- Review comments: drop the redundant ones, document the public API by @fujiwara in https://github.com/sacloud/sakumock/pull/163
- objectstorage: enforce access key quotas (root and permission) by @fujiwara in https://github.com/sacloud/sakumock/pull/165
- CLAUDE.md: how to check what the Terraform provider exposes by @tokuhirom in https://github.com/sacloud/sakumock/pull/168
- Return "" from TestURL on a listener-less server instead of panicking by @fujiwara in https://github.com/sacloud/sakumock/pull/169
- Categorize auto-generated release notes by label by @fujiwara in https://github.com/sacloud/sakumock/pull/172

## [v0.8.0](https://github.com/sacloud/sakumock/compare/v0.7.2...v0.8.0) - 2026-08-13
- objectstorage: authenticate control-plane access keys on the S3 data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/153
- iam: validate responses against the OpenAPI spec and fix service policy endpoints by @fujiwara in https://github.com/sacloud/sakumock/pull/155
- core: share spec-violation inspection routes; roll response validation out to kms by @fujiwara in https://github.com/sacloud/sakumock/pull/156
- Roll response validation out to all remaining services by @fujiwara in https://github.com/sacloud/sakumock/pull/157
- simplenotification: implement history, sources, status, and routing reorder by @fujiwara in https://github.com/sacloud/sakumock/pull/159
- Add CloudHSM mock service by @tokuhirom in https://github.com/sacloud/sakumock/pull/158
- workflows: omit MonthAppliedPlan when unsubscribed by @fujiwara in https://github.com/sacloud/sakumock/pull/160
- apprun: order listings by creation sequence, not timestamp alone by @fujiwara in https://github.com/sacloud/sakumock/pull/161
- iam: back the two-factor endpoints with real state by @fujiwara in https://github.com/sacloud/sakumock/pull/162

## [v0.7.2](https://github.com/sacloud/sakumock/compare/v0.7.1...v0.7.2) - 2026-08-04
- Update sacloud-sdk-go to v0.0.1 by @fujiwara in https://github.com/sacloud/sakumock/pull/149
- build(deps): Bump google.golang.org/grpc from 1.81.1 to 1.82.1 by @dependabot[bot] in https://github.com/sacloud/sakumock/pull/150

## [v0.7.1](https://github.com/sacloud/sakumock/compare/v0.7.0...v0.7.1) - 2026-07-20
- Fix Windows compatibility issues by @fujiwara in https://github.com/sacloud/sakumock/pull/146
- feat: fault injection by @fujiwara in https://github.com/sacloud/sakumock/pull/148

## [v0.7.0](https://github.com/sacloud/sakumock/compare/v0.6.0...v0.7.0) - 2026-07-04
- Generate request-body validation from OpenAPI specs by @fujiwara in https://github.com/sacloud/sakumock/pull/128
- Roll out generated spec-derived validation to six services by @fujiwara in https://github.com/sacloud/sakumock/pull/130
- Roll out generated spec-derived validation to the remaining four services by @fujiwara in https://github.com/sacloud/sakumock/pull/131
- Keep empty-string guards for identifiers the spec gives no minLength by @fujiwara in https://github.com/sacloud/sakumock/pull/132
- Declare no-minLength spec gaps via core.WithNonEmpty overrides by @fujiwara in https://github.com/sacloud/sakumock/pull/133
- feat(apigw): add API Gateway mock service (control plane) by @fujiwara in https://github.com/sacloud/sakumock/pull/134
- feat(apigw): add the gateway data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/136
- build(deps): Bump golang.org/x/net from 0.54.0 to 0.55.0 by @dependabot[bot] in https://github.com/sacloud/sakumock/pull/135
- feat(apigw): enforce data-plane authentication, group ACL, and IP restriction by @fujiwara in https://github.com/sacloud/sakumock/pull/137
- feat(core): log the error reason on 4xx/5xx responses in every service by @fujiwara in https://github.com/sacloud/sakumock/pull/138
- feat(apigw): validate OIDC Bearer tokens on the data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/139
- feat(apigw): support the OIDC authorization code flow with sessions by @fujiwara in https://github.com/sacloud/sakumock/pull/140
- feat(apigw): apply request/response transformations on the data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/141
- feat(apigw): serve object storage backends on the data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/142
- feat(apigw): enforce corsConfig on the data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/143
- chore: drop what-only comments and fix two stale ones by @fujiwara in https://github.com/sacloud/sakumock/pull/144
- feat: OpenTelemetry trace support by @fujiwara in https://github.com/sacloud/sakumock/pull/145

## [v0.6.0](https://github.com/sacloud/sakumock/compare/v0.5.2...v0.6.0) - 2026-06-28
- feat: add Workflows service mock (control plane) by @fujiwara in https://github.com/sacloud/sakumock/pull/119
- feat: add Workflows data plane (Runbook execution engine) by @fujiwara in https://github.com/sacloud/sakumock/pull/121
- docs: add workflows service to root README by @fujiwara in https://github.com/sacloud/sakumock/pull/122
- feat: add service link (EventBus → SimpleMQ / SimpleNotification) by @fujiwara in https://github.com/sacloud/sakumock/pull/123
- docs(eventbus): add service link section by @fujiwara in https://github.com/sacloud/sakumock/pull/125
- feat: add InspectionClient for mock-only endpoints by @fujiwara in https://github.com/sacloud/sakumock/pull/126
- docs: update READMEs to match implemented endpoints by @fujiwara in https://github.com/sacloud/sakumock/pull/127

## [v0.5.2](https://github.com/sacloud/sakumock/compare/v0.5.1...v0.5.2) - 2026-06-20
- fix(apprun): use correct status enum value UnHealthy by @fujiwara in https://github.com/sacloud/sakumock/pull/117

## [v0.5.1](https://github.com/sacloud/sakumock/compare/v0.5.0...v0.5.1) - 2026-06-20
- fix(apprundedicated): pass cmd to docker run by @fujiwara in https://github.com/sacloud/sakumock/pull/115
- fix(dataplane): bundle Docker CLI for AppRun data planes by @fujiwara in https://github.com/sacloud/sakumock/pull/114

## [v0.5.0](https://github.com/sacloud/sakumock/compare/v0.4.0...v0.5.0) - 2026-06-20
- feat(iam): add IAM mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/105
- refactor: consolidate JSON/time helpers into core by @fujiwara in https://github.com/sacloud/sakumock/pull/107
- add .gitignore by @fujiwara in https://github.com/sacloud/sakumock/pull/108
- feat: add AppRun mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/109
- docs(iam): add README by @fujiwara in https://github.com/sacloud/sakumock/pull/110
- feat(apprundedicated): add AppRun Dedicated mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/111
- refactor(terraform): split main.tf into per-service files by @fujiwara in https://github.com/sacloud/sakumock/pull/112
- refactor(core): use time-based IDGenerator default base by @fujiwara in https://github.com/sacloud/sakumock/pull/113

## [v0.4.0](https://github.com/sacloud/sakumock/compare/v0.3.0...v0.4.0) - 2026-06-14
- Add eventbus mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/96
- Add object storage mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/98
- Add `sakumock env --export` by @fujiwara in https://github.com/sacloud/sakumock/pull/99
- Add a versitygw-bundled image with the data plane enabled by @fujiwara in https://github.com/sacloud/sakumock/pull/100
- Add an optional telemetry data plane to monitoringsuite by @fujiwara in https://github.com/sacloud/sakumock/pull/101
- Add a common TLS option for all control planes and data planes by @fujiwara in https://github.com/sacloud/sakumock/pull/102
- Enable the Monitoring Suite data plane in the dataplane image by @fujiwara in https://github.com/sacloud/sakumock/pull/103
- feat(eventbus): fire schedules and triggers via a data plane by @fujiwara in https://github.com/sacloud/sakumock/pull/104

## [v0.3.0](https://github.com/sacloud/sakumock/compare/v0.2.1...v0.3.0) - 2026-06-12
- Add Monitoring Suite mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/82
- Tag every log line with the originating service name by @fujiwara in https://github.com/sacloud/sakumock/pull/85
- Add root/test-terraform Makefiles and refresh README by @fujiwara in https://github.com/sacloud/sakumock/pull/91
- Consolidate into a single Go module by @fujiwara in https://github.com/sacloud/sakumock/pull/94
- Migrate to sacloud-sdk-go and consolidate into a single Go module by @fujiwara in https://github.com/sacloud/sakumock/pull/93
- Provide a multi-platform container image on ghcr.io by @fujiwara in https://github.com/sacloud/sakumock/pull/95

## [v0.2.1](https://github.com/sacloud/sakumock/compare/v0.2.0...v0.2.1) - 2026-06-06
- ci: publish releases via a draft so assets attach under immutable releases by @fujiwara in https://github.com/sacloud/sakumock/pull/80

## [v0.2.0](https://github.com/sacloud/sakumock/compare/v0.1.0...v0.2.0) - 2026-06-06
- Update GitHub Actions to latest pinned versions by @fujiwara in https://github.com/sacloud/sakumock/pull/64
- Let tagpr create the GitHub Release with generated notes by @fujiwara in https://github.com/sacloud/sakumock/pull/66
- simplemq: implement control plane API by @fujiwara in https://github.com/sacloud/sakumock/pull/67
- core: add shared IDGenerator for control-plane resource IDs by @fujiwara in https://github.com/sacloud/sakumock/pull/69
- Add "sakumock all" with one-process startup and config file by @fujiwara in https://github.com/sacloud/sakumock/pull/72
- Verify the suite works with Terraform end-to-end (all 4 services) by @fujiwara in https://github.com/sacloud/sakumock/pull/75
- Share the SAKURA standard error envelope as core.StandardError by @fujiwara in https://github.com/sacloud/sakumock/pull/76
- all: share one ID generator across services for globally-unique IDs by @fujiwara in https://github.com/sacloud/sakumock/pull/77
- chore: bump core dependency to v0.0.4 in all services by @fujiwara in https://github.com/sacloud/sakumock/pull/78
- chore: bump service dependencies to their latest releases by @fujiwara in https://github.com/sacloud/sakumock/pull/79

## [v0.0.5](https://github.com/sacloud/sakumock/compare/simplenotification/v0.0.4...simplenotification/v0.0.5) - 2026-06-06
- Pin root module to released service versions by @fujiwara in https://github.com/sacloud/sakumock/pull/61
- Run tagpr only for modules changed in the push by @fujiwara in https://github.com/sacloud/sakumock/pull/63
- Update GitHub Actions to latest pinned versions by @fujiwara in https://github.com/sacloud/sakumock/pull/64
- Let tagpr create the GitHub Release with generated notes by @fujiwara in https://github.com/sacloud/sakumock/pull/66
- simplemq: implement control plane API by @fujiwara in https://github.com/sacloud/sakumock/pull/67
- core: add shared IDGenerator for control-plane resource IDs by @fujiwara in https://github.com/sacloud/sakumock/pull/69
- Add "sakumock all" with one-process startup and config file by @fujiwara in https://github.com/sacloud/sakumock/pull/72
- Verify the suite works with Terraform end-to-end (all 4 services) by @fujiwara in https://github.com/sacloud/sakumock/pull/75
- Share the SAKURA standard error envelope as core.StandardError by @fujiwara in https://github.com/sacloud/sakumock/pull/76
- all: share one ID generator across services for globally-unique IDs by @fujiwara in https://github.com/sacloud/sakumock/pull/77
- chore: bump core dependency to v0.0.4 in all services by @fujiwara in https://github.com/sacloud/sakumock/pull/78

## [v0.0.4](https://github.com/sacloud/sakumock/compare/core/v0.0.3...core/v0.0.4) - 2026-06-06
- Pin root module to released service versions by @fujiwara in https://github.com/sacloud/sakumock/pull/61
- Run tagpr only for modules changed in the push by @fujiwara in https://github.com/sacloud/sakumock/pull/63
- Update GitHub Actions to latest pinned versions by @fujiwara in https://github.com/sacloud/sakumock/pull/64
- Let tagpr create the GitHub Release with generated notes by @fujiwara in https://github.com/sacloud/sakumock/pull/66
- simplemq: implement control plane API by @fujiwara in https://github.com/sacloud/sakumock/pull/67
- core: add shared IDGenerator for control-plane resource IDs by @fujiwara in https://github.com/sacloud/sakumock/pull/69
- Add "sakumock all" with one-process startup and config file by @fujiwara in https://github.com/sacloud/sakumock/pull/72
- Verify the suite works with Terraform end-to-end (all 4 services) by @fujiwara in https://github.com/sacloud/sakumock/pull/75
- Share the SAKURA standard error envelope as core.StandardError by @fujiwara in https://github.com/sacloud/sakumock/pull/76
- all: share one ID generator across services for globally-unique IDs by @fujiwara in https://github.com/sacloud/sakumock/pull/77

## [v0.1.0](https://github.com/sacloud/sakumock/compare/v0.0.1...v0.1.0) - 2026-05-29
- Add --routes flag to list supported HTTP endpoints by @fujiwara in https://github.com/sacloud/sakumock/pull/27
- Move per-module tagpr configs into each subdirectory by @fujiwara in https://github.com/sacloud/sakumock/pull/29
- Pin services to released core v0.0.1 by @fujiwara in https://github.com/sacloud/sakumock/pull/32
- [kms] Release for v0.0.3 by @github-actions[bot] in https://github.com/sacloud/sakumock/pull/20
- Add HTTP rate limit option to all services by @fujiwara in https://github.com/sacloud/sakumock/pull/38
- bump core to v0.0.2 by @fujiwara in https://github.com/sacloud/sakumock/pull/45
- Add unified sakumock binary with per-service subcommands by @fujiwara in https://github.com/sacloud/sakumock/pull/52
- Pin root module to released service versions by @fujiwara in https://github.com/sacloud/sakumock/pull/61
- Run tagpr only for modules changed in the push by @fujiwara in https://github.com/sacloud/sakumock/pull/63

## [v0.0.4](https://github.com/sacloud/sakumock/compare/simplenotification/v0.0.3...simplenotification/v0.0.4) - 2026-05-29
- Add unified sakumock binary with per-service subcommands by @fujiwara in https://github.com/sacloud/sakumock/pull/52

## [v0.0.3](https://github.com/sacloud/sakumock/compare/core/v0.0.2...core/v0.0.3) - 2026-05-29
- bump core to v0.0.2 by @fujiwara in https://github.com/sacloud/sakumock/pull/45
- Add unified sakumock binary with per-service subcommands by @fujiwara in https://github.com/sacloud/sakumock/pull/52

## [v0.0.3](https://github.com/sacloud/sakumock/compare/simplenotification/v0.0.2...simplenotification/v0.0.3) - 2026-05-20
- bump core to v0.0.2 by @fujiwara in https://github.com/sacloud/sakumock/pull/45

## [v0.0.2](https://github.com/sacloud/sakumock/compare/simplenotification/v0.0.1...simplenotification/v0.0.2) - 2026-05-02
- [kms] Release for v0.0.3 by @github-actions[bot] in https://github.com/sacloud/sakumock/pull/20
- Add HTTP rate limit option to all services by @fujiwara in https://github.com/sacloud/sakumock/pull/38

## [v0.0.2](https://github.com/sacloud/sakumock/compare/core/v0.0.1...core/v0.0.2) - 2026-05-02
- Pin services to released core v0.0.1 by @fujiwara in https://github.com/sacloud/sakumock/pull/32
- [kms] Release for v0.0.3 by @github-actions[bot] in https://github.com/sacloud/sakumock/pull/20
- Add HTTP rate limit option to all services by @fujiwara in https://github.com/sacloud/sakumock/pull/38

## [v0.0.1](https://github.com/sacloud/sakumock/compare/core/v0.0.1...simplenotification/v0.0.1) - 2026-05-01
- Pin services to released core v0.0.1 by @fujiwara in https://github.com/sacloud/sakumock/pull/32

## [v0.0.1](https://github.com/sacloud/sakumock/compare/v0.0.1...core/v0.0.1) - 2026-05-01
- Add --routes flag to list supported HTTP endpoints by @fujiwara in https://github.com/sacloud/sakumock/pull/27
- Move per-module tagpr configs into each subdirectory by @fujiwara in https://github.com/sacloud/sakumock/pull/29

## [v0.0.1](https://github.com/sacloud/sakumock/compare/simplemq/v0.0.2...v0.0.1) - 2026-05-01
- Split changelog into per-service files by @fujiwara in https://github.com/sacloud/sakumock/pull/21
- Fix tagpr config: use changelogFile instead of changelog by @fujiwara in https://github.com/sacloud/sakumock/pull/23
- Add simplenotification mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/24
- Clarify CLAUDE.md error-schema rule for endpoints without a spec definition by @fujiwara in https://github.com/sacloud/sakumock/pull/26
