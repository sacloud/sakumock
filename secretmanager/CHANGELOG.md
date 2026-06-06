# Changelog

## [v0.1.0](https://github.com/sacloud/sakumock/compare/secretmanager/v0.0.6...secretmanager/v0.1.0) - 2026-06-06
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

## [v0.0.6](https://github.com/sacloud/sakumock/compare/secretmanager/v0.0.5...secretmanager/v0.0.6) - 2026-05-29
- Add unified sakumock binary with per-service subcommands by @fujiwara in https://github.com/sacloud/sakumock/pull/52

## [v0.0.5](https://github.com/sacloud/sakumock/compare/secretmanager/v0.0.4...secretmanager/v0.0.5) - 2026-05-20
- bump core to v0.0.2 by @fujiwara in https://github.com/sacloud/sakumock/pull/45

## [v0.0.4](https://github.com/sacloud/sakumock/compare/secretmanager/v0.0.3...secretmanager/v0.0.4) - 2026-05-02
- Add HTTP rate limit option to all services by @fujiwara in https://github.com/sacloud/sakumock/pull/38

## [v0.0.3](https://github.com/sacloud/sakumock/compare/secretmanager/v0.0.2...secretmanager/v0.0.3) - 2026-05-01
- Split changelog into per-service files by @fujiwara in https://github.com/sacloud/sakumock/pull/21
- Fix tagpr config: use changelogFile instead of changelog by @fujiwara in https://github.com/sacloud/sakumock/pull/23
- Add simplenotification mock service by @fujiwara in https://github.com/sacloud/sakumock/pull/24
- Clarify CLAUDE.md error-schema rule for endpoints without a spec definition by @fujiwara in https://github.com/sacloud/sakumock/pull/26
- Add --routes flag to list supported HTTP endpoints by @fujiwara in https://github.com/sacloud/sakumock/pull/27
- Move per-module tagpr configs into each subdirectory by @fujiwara in https://github.com/sacloud/sakumock/pull/29
- Pin services to released core v0.0.1 by @fujiwara in https://github.com/sacloud/sakumock/pull/32
- [kms] Release for v0.0.3 by @github-actions[bot] in https://github.com/sacloud/sakumock/pull/20
