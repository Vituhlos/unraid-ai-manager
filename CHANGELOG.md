# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.7] - 2026-06-28

### Fixed

- Fixed plugin upgrades leaving the previous helper process running by restarting the helper after package installation instead of calling `start`.
- Ensured `/v1/capabilities` reports the newly installed helper version immediately after upgrading the Unraid plugin.

## [0.1.6] - 2026-06-28

### Fixed

- Fixed Unraid rc script packaging from Windows release runners by enforcing LF line endings for plugin shell/PHP/page files.
- Added package-time LF normalization for the packaged rc script and plugin UI files.
- Added plugin validation that inspects the generated `.txz` and fails if `rc.unraid-ai-manager` contains CRLF/CR line endings.

## [0.1.5] - 2026-06-28

### Added

- Added a generic dashboard planning model (`dashboard-config`) so AMUD is now the first dashboard provider adapter instead of the only dashboard concept in the architecture.
- Added helper API endpoints `POST /v1/plan/dashboard` and `POST /v1/apply/dashboard`.
- Added helper API endpoint `GET /v1/capabilities` so AI clients can discover implemented and planned safe action modules.
- Added MCP tools `unraid_capabilities`, `unraid_plan_dashboard` and `unraid_apply_dashboard`.
- Added CLI commands `plan-dashboard` and `apply-dashboard-plan`.
- Added service metadata inference in dashboard plans, including display name, slug, icon, category and likely integration type.
- Added tests for the generic dashboard planner and AMUD adapter helper flow.
- Added GitHub Actions CI for Go tests, MCP syntax checks, Unraid plugin packaging and plugin validation.
- Added GitHub Actions release automation for `vMAJOR.MINOR.PATCH` tags and manual release dispatches.

### Changed

- `unraid_plan_amud`, `unraid_apply_amud`, `plan-amud` and `apply-amud-plan` are now compatibility shortcuts. New workflows should use the generic dashboard APIs and tools with `provider=amud`.
- Documentation now describes Unraid AI Manager as a general Unraid automation control plane with dashboard provider adapters.

## [0.1.4] - 2026-06-28

### Added

- Added an approved Docker recreate apply workflow that calls Unraid DockerMan `rebuild_container` from a whitelist instead of generating raw Docker commands.
- Added `POST /v1/apply/recreate` to the helper API and `unraid_apply_recreate` to the MCP server.
- Added `apply-recreate-plan` to the CLI.
- Recreate apply now records an audit log, verifies runtime state through Docker inspect, and starts a container again when it was running before the rebuild but DockerMan leaves it stopped.

### Security

- Recreate apply validates container names and the DockerMan rebuild script path before executing anything.
- Recreate apply still requires the exact plan hash and, when enabled, a short-lived approval token.

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

[Unreleased]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.7...HEAD
[0.1.7]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Vituhlos/unraid-ai-manager/releases/tag/v0.1.0
