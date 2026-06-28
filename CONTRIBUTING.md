# Contributing

This project is early-stage and safety-sensitive. Changes should be small, reviewable and explicit about user impact.

## Development rules

- Prefer Go for production CLI/helper code.
- Keep the MCP server narrow: named tools only, no raw shell passthrough.
- Do not add write operations without a plan, diff, confirmation and audit path.
- Do not introduce broad Docker socket or filesystem access without a security design update.
- Keep English and Czech documentation in sync when user-facing behavior changes.

## Commit messages

Use concise Conventional Commit-style messages.

Preferred format:

```text
type(scope): short description

EN: User-facing explanation in English.
CS: Uživatelské vysvětlení česky.
```

Examples:

```text
docs: add bilingual changelog and versioning policy

EN: Adds Keep a Changelog-based release notes and a SemVer policy.
CS: Přidává release notes podle Keep a Changelog a politiku SemVer verzování.
```

```text
feat(helper): require approval tokens for apply endpoints

EN: Adds a short-lived local approval gate before helper apply operations.
CS: Přidává krátkodobou lokální schvalovací bránu před apply operace helperu.
```

Allowed types:

- `feat`: user-visible feature
- `fix`: bug fix
- `docs`: documentation-only change
- `security`: security hardening or vulnerability fix
- `refactor`: internal restructuring without behavior change
- `test`: tests
- `build`: build, packaging or release automation
- `chore`: repository maintenance

## Changelog rules

The changelog follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).

- Update `CHANGELOG.md` and `CHANGELOG.cs.md` for every user-visible change.
- Keep an `Unreleased` section at the top.
- Use ISO dates: `YYYY-MM-DD`.
- Group changes under `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` and `Security`.
- Do not dump git logs into the changelog.

## Pull request checklist

- [ ] Tests pass.
- [ ] MCP tool schema changes are documented.
- [ ] Helper endpoint changes are documented.
- [ ] Security implications are described.
- [ ] `CHANGELOG.md` and `CHANGELOG.cs.md` are updated when needed.
- [ ] `VERSION` is updated only when preparing a release.
