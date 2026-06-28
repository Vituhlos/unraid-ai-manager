# Agent Instructions

These instructions are for AI agents working on this repository.

## Documentation language policy

User-facing documentation must be maintained in English and Czech:

- English: `*.md`
- Czech: `*.cs.md`

When changing user-visible behavior, update both language versions in the same commit.

## Changelog policy

Follow [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).

- Update `CHANGELOG.md` and `CHANGELOG.cs.md` for user-visible changes.
- Keep `Unreleased` at the top.
- Use `YYYY-MM-DD` release dates.
- Use the standard sections: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.
- Do not generate changelog entries by dumping git commits.

## Versioning policy

Follow [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

- Read the current version from `VERSION`.
- Git tags use `vMAJOR.MINOR.PATCH`.
- Unraid plugin/package versions use `MAJOR.MINOR.PATCH`.
- Do not modify published release artifacts in place.

## Security policy

This project controls privileged Unraid operations. Preserve these invariants:

- no raw shell passthrough from MCP;
- no unrestricted Docker socket access from MCP;
- no write operation without plan, diff, confirmation and audit;
- no container lifecycle write operation without explicit policy and approval;
- no dangerous mounts or privileged containers without a separate high-risk approval design.

## Commit message policy

Use concise Conventional Commit-style messages with bilingual body when helpful:

```text
type(scope): short description

EN: English user-facing explanation.
CS: České uživatelské vysvětlení.
```
