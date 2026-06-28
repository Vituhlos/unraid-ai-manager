# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.3] - 2026-06-28

### Changed

- AMUD planning now defaults to templates that define a DockerMan `WebUI`, instead of also treating every TCP-only service as a web application.
- Port-only AMUD candidates can still be included explicitly with `include_port_only` in the helper/MCP API or `--include-port-only` in the CLI.
- Helper/MCP AMUD planning now defaults to `runtime_filter=running` when Docker runtime access is configured, so stale DockerMan XML templates are not planned by default.
- Release builds now generate Linux/Unraid binaries by default; Windows binaries require an explicit build flag and are not published until code signing and false-positive handling are improved.

### Added

- Added AMUD include and exclude filters for helper/MCP planning requests via `containers` and `exclude_containers`.
- Added AMUD include and exclude filters to the CLI via repeated `--container` and `--exclude` flags.
- Added AMUD runtime filtering via `runtime_filter=templates|existing|running`.
- Added tests for port-only AMUD filtering and container include/exclude behavior.

## [0.1.2] - 2026-06-28

### Fixed

- Fixed the Unraid Settings page rendering as blank by adding the required `.page` content separator and explicitly including the plugin PHP UI file.
- Reduced the risk of PHP helper name collisions with Unraid/Dynamix by prefixing plugin UI helper functions with `uaim_`.

### Added

- Added a plugin validation script that checks `.page` structure, version consistency and generated `.plg` release URLs before publishing.

## [0.1.1] - 2026-06-28

### Added

- English and Czech README files.
- English and Czech changelog files following Keep a Changelog 1.1.0.
- English and Czech versioning policy based on Semantic Versioning 2.0.0.
- English and Czech security policy.
- English and Czech contributing guide with commit message conventions.
- Release checklist for GitHub and Unraid plugin releases.
- Canonical `VERSION` file.
- Planning documentation for future Community Applications installation support.
- Release governance documentation for commit messages, version bumps and release checklists.

### Changed

- Updated the Unraid plugin packaging script to use the canonical SemVer version from `VERSION` by default.
- Standardized GitHub release tags as `vMAJOR.MINOR.PATCH`.
- Standardized Unraid plugin/package versions as `MAJOR.MINOR.PATCH`.
- Reworked the README to describe the current helper, MCP and approval-token workflow accurately.
- The documentation set is now maintained in English and Czech.

## [0.1.0] - 2026-06-28

### Added

- Initial Go CLI for reading Unraid DockerMan XML templates.
- Initial Unraid-side helper daemon with HTTP endpoints for health, inventory, Docker inspect, runtime comparison, AMUD planning, TZ planning, recreate planning, AMUD apply, TZ apply and XML restore.
- Initial PC-side MCP server exposing whitelisted Unraid tools.
- XML backup, audit log and rollback support.
- Hash-confirmed AMUD label application workflow.
- Hash-confirmed TZ environment variable application workflow.
- Optional short-lived approval token model for apply operations.
- Read-only Docker API client for container inventory and inspect data.
- Runtime comparison between DockerMan XML templates and live Docker configuration.
- Read-only recreate planner.
- Unraid plugin packaging with Settings page, rc script and appdata-backed configuration.
- Windows and Linux build scripts.
- Python reference prototype for early XML planning experiments.

### Security

- MCP tools do not accept raw shell commands.
- Apply operations require an exact plan hash.
- XML restore requires an exact backup SHA-256 hash.
- The helper binds to `127.0.0.1:37231` by default.
- The helper supports API-key authentication.

[Unreleased]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Vituhlos/unraid-ai-manager/releases/tag/v0.1.0
