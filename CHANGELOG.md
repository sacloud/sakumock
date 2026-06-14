# Changelog

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
