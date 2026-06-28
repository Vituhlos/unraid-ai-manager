# Release Checklist

Use this checklist for every GitHub and Unraid plugin release.

## 1. Prepare

- [ ] Decide the new SemVer version.
- [ ] Update `VERSION`.
- [ ] Move relevant `CHANGELOG.md` entries from `Unreleased` to the new version section.
- [ ] Mirror the changelog update in `CHANGELOG.cs.md`.
- [ ] Verify README and versioning docs still match the release.

## 2. Validate

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" test ./...
node --check .\mcp\unraid-mcp-server.mjs
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\validate-unraid-plugin.ps1
```

Verify package contents:

```powershell
tar -tf .\dist\unraid-ai-manager-<version>-x86_64-1.txz
```

## 3. Commit and tag

```powershell
git status -sb
git add .
git commit -m "release: v<version>" -m "EN: Prepare release v<version>." -m "CS: Připravuje release v<version>."
git tag v<version>
git push origin main
git push origin v<version>
```

## 4. Publish GitHub release

Create the release from the changelog entry. Upload:

- `dist/unraid-ai-manager.plg`
- `dist/unraid-ai-manager-<version>-x86_64-1.txz`
- `dist/unraid-ai-manager-<version>-x86_64-1.txz.sha256`
- Linux binaries and checksums
- Windows binaries and checksums

## 5. Verify install URLs

```powershell
Invoke-WebRequest `
  -Uri "https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg" `
  -MaximumRedirection 5 `
  -UseBasicParsing
```

The response should contain:

```text
<PLUGIN name=
```

## 6. Smoke test on Unraid

- [ ] Install plugin from the release URL.
- [ ] Open `Settings -> Unraid AI Manager`.
- [ ] Generate an API key.
- [ ] Start helper.
- [ ] Verify `/v1/health`.
- [ ] Verify inventory through MCP.
- [ ] Do not test write operations on production containers without reviewing the diff and backup location.
