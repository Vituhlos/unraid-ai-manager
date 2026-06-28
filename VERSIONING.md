# Versioning Policy

Unraid AI Manager follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

## Canonical version

The canonical project version is stored in:

```text
VERSION
```

The value must be a SemVer version without the leading `v`, for example:

```text
0.1.1
```

## Tags and artifacts

- GitHub release tags use `vMAJOR.MINOR.PATCH`, for example `v0.1.1`.
- Unraid plugin versions use `MAJOR.MINOR.PATCH`, for example `0.1.1`.
- Unraid package names use `unraid-ai-manager-MAJOR.MINOR.PATCH-x86_64-1.txz`.
- Release notes are copied from [CHANGELOG.md](CHANGELOG.md).

## Version bump rules while `0.y.z`

The project is currently in the `0.y.z` initial development phase. APIs, MCP tools and helper endpoints may still change.

- `PATCH` bump: documentation corrections, packaging fixes, internal cleanup, small backward-compatible fixes.
- `MINOR` bump: new user-visible capability, new MCP tool, new helper endpoint, new apply workflow, new plugin UI capability.
- `MAJOR` bump: reserved for `1.0.0` or for a clearly intentional compatibility reset.

## Version bump rules after `1.0.0`

- `PATCH`: backward-compatible bug fixes.
- `MINOR`: backward-compatible functionality.
- `MAJOR`: backward-incompatible public API, MCP tool or helper endpoint changes.

## Release immutability

Published release artifacts must not be modified in place. If an artifact is wrong, create a new version.

Allowed exception: GitHub release notes may be edited to fix typos or add clarifying links, but not to hide functional changes.

## Pre-release versions

Pre-release tags may be used later:

```text
v0.2.0-alpha.1
v0.2.0-rc.1
```

Unraid plugin releases should prefer stable tags unless a release is clearly marked as experimental.
