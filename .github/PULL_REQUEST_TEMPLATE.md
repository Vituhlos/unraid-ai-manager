## Summary / Shrnutí

- EN:
- CS:

## Safety impact / Bezpečnostní dopad

- [ ] No new write operation.
- [ ] New write operation has plan, diff, confirmation, backup and audit.
- [ ] No raw shell passthrough was added.
- [ ] Docker socket/filesystem exposure did not expand.

## Documentation / Dokumentace

- [ ] English docs updated where needed.
- [ ] Czech docs updated where needed.
- [ ] `CHANGELOG.md` updated where needed.
- [ ] `CHANGELOG.cs.md` updated where needed.

## Validation / Ověření

- [ ] `go test ./...`
- [ ] `node --check mcp/unraid-mcp-server.mjs`
- [ ] Plugin package built when packaging changed.
