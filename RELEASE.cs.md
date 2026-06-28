# Release checklist

Tento checklist používej pro každý GitHub a Unraid plugin release.

## 1. Příprava

- [ ] Rozhodnout novou SemVer verzi.
- [ ] Aktualizovat `VERSION`.
- [ ] Přesunout relevantní položky v `CHANGELOG.md` z `Unreleased` do nové verze.
- [ ] Stejnou změnu promítnout do `CHANGELOG.cs.md`.
- [ ] Ověřit, že README a versioning dokumentace odpovídají releasu.

## 2. Validace

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" test ./...
node --check .\mcp\unraid-mcp-server.mjs
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1
```

Ověření obsahu balíku:

```powershell
tar -tf .\dist\unraid-ai-manager-<version>-x86_64-1.txz
```

## 3. Commit a tag

```powershell
git status -sb
git add .
git commit -m "release: v<version>" -m "EN: Prepare release v<version>." -m "CS: Připravuje release v<version>."
git tag v<version>
git push origin main
git push origin v<version>
```

## 4. Publikace GitHub releasu

Release notes vezmi z changelogu. Nahraj:

- `dist/unraid-ai-manager.plg`
- `dist/unraid-ai-manager-<version>-x86_64-1.txz`
- `dist/unraid-ai-manager-<version>-x86_64-1.txz.sha256`
- Linux binárky a checksumy
- Windows binárky a checksumy

## 5. Ověření install URL

```powershell
Invoke-WebRequest `
  -Uri "https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg" `
  -MaximumRedirection 5 `
  -UseBasicParsing
```

Odpověď má obsahovat:

```text
<PLUGIN name=
```

## 6. Smoke test na Unraidu

- [ ] Nainstalovat plugin z release URL.
- [ ] Otevřít `Settings -> Unraid AI Manager`.
- [ ] Vygenerovat API key.
- [ ] Spustit helper.
- [ ] Ověřit `/v1/health`.
- [ ] Ověřit inventory přes MCP.
- [ ] Netestovat write operace na produkčních kontejnerech bez kontroly diffu a backup lokace.
