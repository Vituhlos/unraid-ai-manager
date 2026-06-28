# Unraid AI Manager

Bezpečná automatizace Unraidu pro AI asistenty.

Unraid AI Manager je lokální control-plane pro správu Unraid DockerMan šablon přes přísný workflow plán → diff → schválení → aplikace. Je navržený pro AI/MCP klienty, ale bezpečnostní hranice není chat s AI. Bezpečnostní hranice je helper daemon běžící lokálně na Unraidu.

> Aktuální stav: `v0.1.1` je rané preview. Umí načíst DockerMan XML šablony, přečíst runtime stav Dockeru, naplánovat AMUD/TZ/template změny, aplikovat schválené XML úpravy se zálohami a auditem a vystavit tyto akce přes MCP server. Instalace Community Applications a reálné lifecycle akce kontejnerů jsou zatím plánované, ne implementované.

## Jazyky

- Čeština: tento soubor
- Angličtina: [README.md](README.md)
- Architektura: [DESIGN.md](DESIGN.md) / [DESIGN.en.md](DESIGN.en.md)
- Changelog: [CHANGELOG.cs.md](CHANGELOG.cs.md) / [CHANGELOG.md](CHANGELOG.md)
- Verzování: [VERSIONING.cs.md](VERSIONING.cs.md) / [VERSIONING.md](VERSIONING.md)
- Bezpečnost: [SECURITY.cs.md](SECURITY.cs.md) / [SECURITY.md](SECURITY.md)
- Přispívání a commity: [CONTRIBUTING.cs.md](CONTRIBUTING.cs.md) / [CONTRIBUTING.md](CONTRIBUTING.md)

## Co nástroj umí

- Čte Unraid DockerMan XML šablony z `/boot/config/plugins/dockerMan/templates-user`.
- Parsuje porty, cesty, proměnné, labely, WebUI, metadata šablony a repository.
- Čte runtime stav Dockeru přes read-only Docker API volání.
- Porovnává DockerMan XML šablony s živou konfigurací kontejnerů.
- Navrhuje AMUD labely:
  - `amud.enable=true`
  - `amud.url=...`
  - `amud.name=...`
  - `amud.icon=...`
- Podporuje AMUD URL režimy:
  - `local`: `http://<local_host>:<host_port>`
  - `cloudflare`: `https://<subdomain>.<domain>`
  - `hybrid`: Cloudflare, pokud existuje route, jinak local
- Plánuje a aplikuje změny env proměnné `TZ`.
- Před každým zápisem vytváří XML backup.
- Před aplikací vyžaduje hash plánu.
- Volitelně vyžaduje krátkodobý lokální approval token.
- Zapisuje audit logy.
- Umí obnovit XML šablonu z ověřené zálohy.
- Vystavuje MCP server na PC pouze s whitelistovanými nástroji.

## Co záměrně zatím neumí

- Zatím neinstaluje kontejnery z Community Applications.
- Zatím nerecreatuje, nestartuje, nezastavuje ani nemaže kontejnery.
- Nepřijímá raw shell příkazy od AI.
- Nevystavuje neomezený Docker socket MCP klientům.
- Nemountuje celý host filesystem do AI řízeného kontejneru.

Tyto schopnosti jsou plánované až za explicitní policy, risk scoring a extra schvalovací brány.

## Architektura

```text
AI klient na PC
  Claude / ChatGPT / jiný MCP klient
        |
        v
MCP server na PC
  mcp/unraid-mcp-server.mjs
        |
        v
Helper daemon na Unraidu
  unraid-ai-helper
        |
        +--> DockerMan XML šablony
        +--> read-only Docker API inspect
        +--> backup / audit / approval token store
```

MCP server převádí požadavky AI na úzké pojmenované nástroje. Helper daemon vynucuje policy a provádí lokální filesystem operace na Unraidu.

## Doporučená instalace

Unraid plugin nainstaluješ z URL:

```text
https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg
```

V Unraidu:

1. Otevři `Plugins`.
2. Vlož URL do `Install Plugin`.
3. Otevři `Settings -> Unraid AI Manager`.
4. Vygeneruj API key.
5. Nech helper bindnutý na `127.0.0.1:37231`, pokud nemáš konkrétní důvod ho vystavit jinak.
6. Z PC se připoj přes SSH tunnel:

```bash
ssh -L 37231:127.0.0.1:37231 root@<unraid-ip>
```

## MCP konfigurace

MCP server běží na PC a připojuje se k helperu na Unraidu přes HTTP.

```powershell
$env:UNRAID_AI_HELPER_URL="http://127.0.0.1:37231"
$env:UNRAID_AI_API_KEY="<vygenerovany-api-key>"
node .\mcp\unraid-mcp-server.mjs
```

Příklad konfigurace MCP klienta:

```json
{
  "mcpServers": {
    "unraid-ai-manager": {
      "command": "node",
      "args": [
        "C:\\path\\to\\unraid-ai-manager\\mcp\\unraid-mcp-server.mjs"
      ],
      "env": {
        "UNRAID_AI_HELPER_URL": "http://127.0.0.1:37231",
        "UNRAID_AI_API_KEY": "<vygenerovany-api-key>"
      }
    }
  }
}
```

Dostupné MCP tools:

- `unraid_health`
- `unraid_inventory`
- `unraid_docker_inspect`
- `unraid_compare_runtime`
- `unraid_plan_amud`
- `unraid_apply_amud`
- `unraid_plan_tz`
- `unraid_apply_tz`
- `unraid_plan_recreate`
- `unraid_restore_xml`

Apply tools vyžadují `confirm_plan_hash`. Pokud jsou zapnuté approval tokeny, vyžadují i `approval_token`.

## Bezpečný AMUD workflow

1. Řekni AI, ať vytvoří plán, ne ať ho rovnou aplikuje.
2. Zkontroluj navržené labely, URL, rizika a XML diff.
3. Potvrď přesný `plan_hash`.
4. Na Unraidu vytvoř krátkodobý lokální approval token:

```bash
unraid-ai-manager approve-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/PLAN.json \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --purpose amud \
  --ttl 15m
```

5. Nech AI zavolat apply tool s hashem plánu a tokenem.
6. Ověř audit log a výsledné XML.

## Ruční CLI příklady

Inventory:

```bash
unraid-ai-manager inventory \
  --templates /boot/config/plugins/dockerMan/templates-user
```

AMUD plán:

```bash
unraid-ai-manager plan-amud \
  --templates /boot/config/plugins/dockerMan/templates-user \
  --url-mode hybrid \
  --local-host 192.0.2.10 \
  --cloudflare-domain example.com \
  --route radarr=radarr \
  --route sonarr=sonarr \
  --diff \
  --out /mnt/user/appdata/unraid-ai-manager/plans/amud-plan.json
```

Aplikace schváleného AMUD plánu:

```bash
unraid-ai-manager apply-amud-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/amud-plan.json \
  --confirm-plan-hash <plan_hash> \
  --approval-token <approval_token> \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --backup-dir /mnt/user/appdata/unraid-ai-manager/backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit
```

Restore XML zálohy:

```bash
unraid-ai-manager restore-xml-backup \
  --backup /mnt/user/appdata/unraid-ai-manager/backups/my-container.xml \
  --target /boot/config/plugins/dockerMan/templates-user/my-container.xml \
  --confirm-backup-sha256 <backup_sha256> \
  --pre-restore-backup-dir /mnt/user/appdata/unraid-ai-manager/restore-backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit
```

## Vývoj

Produkční helper/CLI je v Go. Starší Python část v repozitáři zůstává jako referenční prototyp.

Portable Go může být v:

```text
.tools/go/go1.26.4/go/bin/go.exe
```

Testy:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" test ./...
node --check .\mcp\unraid-mcp-server.mjs
```

Build binárek:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Build Unraid plugin artefaktů:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1
```

Výstupy jsou v `dist/`.

## Release verze

Kanonická verze projektu je v [VERSION](VERSION).

- Git tagy používají `vMAJOR.MINOR.PATCH`.
- Unraid plugin/package verze používají `MAJOR.MINOR.PATCH`.
- Release notes jsou v [CHANGELOG.cs.md](CHANGELOG.cs.md).

Celá politika je ve [VERSIONING.cs.md](VERSIONING.cs.md).

## Licence

Licence zatím není vybraná. Dokud nebude přidaný licenční soubor, všechna práva si ponechává vlastník repozitáře.
