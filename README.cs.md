# Unraid AI Manager

Bezpečná automatizace Unraidu pro AI asistenty.

Unraid AI Manager je lokální control-plane pro správu Unraid DockerMan šablon přes přísný workflow plán → diff → schválení → aplikace. Je navržený pro AI/MCP klienty, ale bezpečnostní hranice není chat s AI. Bezpečnostní hranice je helper daemon běžící lokálně na Unraidu.

> Aktuální stav: rané preview. Poslední vydaný plugin je `v0.1.7`; main branch může obsahovat nevydané workflow změny. Umí načíst DockerMan XML šablony, přečíst runtime stav Dockeru, naplánovat obecné dashboard/TZ/template změny, aplikovat schválené XML úpravy se zálohami a auditem, aplikovat schválené DockerMan recreate plány, bezpečně objevovat známé integrace bez vyzrazení plných secretů a vystavit tyto akce přes MCP server. AMUD je první dashboard adapter. Instalace Community Applications a libovolný lifecycle management kontejnerů jsou zatím plánované, ne implementované.

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
- Vystavuje capability mapu, aby AI klient nejdřív zjistil bezpečné hotové/plánované moduly akcí.
- Parsuje porty, cesty, proměnné, labely, WebUI, metadata šablony a repository.
- Čte runtime stav Dockeru přes read-only Docker API volání.
- Porovnává DockerMan XML šablony s živou konfigurací kontejnerů.
- Objevuje známé integrace aplikací a appdata umístění API klíčů/tokenů s maskovaným náhledem místo plných secret hodnot.
- Discovery vrací stabilní `secret_ref` identifikátory pro secrety. Plné hodnoty secretů se přes MCP nevystavují; budoucí apply workflow je vyřeší interně přímo na Unraidu.
- Plánuje konfiguraci dashboardů přes provider adaptery. První adapter je AMUD přes DockerMan labely:
  - `amud.enable=true`
  - `amud.url=...`
  - `amud.name=...`
  - `amud.icon=...`
- Podporuje dashboard URL režimy:
  - `local`: `http://<local_host>:<host_port>`
  - `cloudflare`: `https://<subdomain>.<domain>`
  - `hybrid`: Cloudflare, pokud existuje route, jinak local
- Defaultně plánuje dashboard záznamy jen pro DockerMan šablony s explicitním `WebUI`.
- Umí explicitně zahrnout TCP port-only šablony nebo omezit/vyloučit konkrétní kontejnery.
- Helper/MCP planning při dostupném Docker runtime přístupu defaultně filtruje na aktuálně běžící Docker containery.
- Plánuje a aplikuje změny env proměnné `TZ`.
- Plánuje a aplikuje schválené Docker recreate operace přes Unraid DockerMan `rebuild_container`.
- Umí spojit dashboard XML apply, DockerMan recreate a runtime ověření do jednoho schváleného dashboard sync workflow.
- Před každým zápisem vytváří XML backup.
- Před aplikací vyžaduje hash plánu.
- Volitelně vyžaduje krátkodobý lokální approval token.
- Zapisuje audit logy.
- Umí obnovit XML šablonu z ověřené zálohy.
- Vystavuje MCP server na PC pouze s whitelistovanými nástroji.

## Co záměrně zatím neumí

- Zatím neinstaluje kontejnery z Community Applications.
- Zatím nedělá libovolné start/stop/remove operace.
- Recreate kontejnerů dělá jen ze schváleného recreate plánu přes Unraid DockerMan.
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
- `unraid_capabilities`
- `unraid_inventory`
- `unraid_docker_inspect`
- `unraid_compare_runtime`
- `unraid_discover_integrations`
- `unraid_plan_dashboard`
- `unraid_apply_dashboard`
- `unraid_plan_dashboard_sync`
- `unraid_apply_dashboard_sync`
- `unraid_plan_amud`
- `unraid_apply_amud`
- `unraid_plan_tz`
- `unraid_apply_tz`
- `unraid_plan_recreate`
- `unraid_apply_recreate`
- `unraid_restore_xml`

Apply tools vyžadují `confirm_plan_hash`. Pokud jsou zapnuté approval tokeny, vyžadují i `approval_token`.

## Bezpečný dashboard workflow

1. Řekni AI, ať vytvoří plán, ne ať ho rovnou aplikuje.
2. Zkontroluj navržený provider, adapter, URL, target changes, rizika a XML diff.
3. Potvrď přesný `plan_hash`.
4. Na Unraidu vytvoř krátkodobý lokální approval token:

```bash
unraid-ai-manager approve-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/PLAN.json \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --purpose dashboard \
  --ttl 15m
```

5. Nech AI zavolat apply tool s hashem plánu a tokenem.
6. Ověř audit log a výsledné XML.
7. Pokud se má změna propsat i do živých kontejnerů, preferuj dashboard sync workflow. To dá XML změny, recreate operace a runtime ověření pod jeden plan hash.

## Ruční CLI příklady

Inventory:

```bash
unraid-ai-manager inventory \
  --templates /boot/config/plugins/dockerMan/templates-user
```

Discovery známých integrací a maskovaných umístění API klíčů/tokenů:

```bash
unraid-ai-manager discover-integrations \
  --templates /boot/config/plugins/dockerMan/templates-user
```

Výstup obsahuje maskované náhledy a `secret_ref` hodnoty. Ber `secret_ref` jako capability referenci, ne jako secret; hodí se jen pro schválené lokální workflow.

Dashboard plán přes AMUD adapter:

```bash
unraid-ai-manager plan-dashboard \
  --provider amud \
  --templates /boot/config/plugins/dockerMan/templates-user \
  --url-mode hybrid \
  --local-host 192.0.2.10 \
  --cloudflare-domain example.com \
  --route radarr=radarr \
  --route sonarr=sonarr \
  --runtime-filter running \
  --docker-socket /var/run/docker.sock \
  --exclude mariadb \
  --exclude mosquitto \
  --diff \
  --out /mnt/user/appdata/unraid-ai-manager/plans/dashboard-plan.json
```

Aplikace schváleného dashboard plánu:

```bash
unraid-ai-manager apply-dashboard-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/dashboard-plan.json \
  --confirm-plan-hash <plan_hash> \
  --backup-dir /mnt/user/appdata/unraid-ai-manager/backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit
```

Dashboard plán i propsání do runtime jako jeden sync workflow:

```bash
unraid-ai-manager plan-dashboard-sync \
  --provider amud \
  --templates /boot/config/plugins/dockerMan/templates-user \
  --url-mode local \
  --local-host 192.0.2.10 \
  --runtime-filter running \
  --recreate-mode changed \
  --docker-socket /var/run/docker.sock \
  --diff \
  --out /mnt/user/appdata/unraid-ai-manager/plans/dashboard-sync-plan.json
```

Aplikace schváleného dashboard sync plánu:

```bash
unraid-ai-manager apply-dashboard-sync-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/dashboard-sync-plan.json \
  --confirm-plan-hash <sync_plan_hash> \
  --backup-dir /mnt/user/appdata/unraid-ai-manager/backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit \
  --docker-socket /var/run/docker.sock
```

`plan-amud` a `apply-amud-plan` zůstávají jako kompatibilní zkratky pro AMUD label adapter, ale nové workflow by mělo používat obecné dashboard příkazy.

Aplikace schváleného recreate plánu:

```bash
unraid-ai-manager apply-recreate-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/recreate-plan.json \
  --confirm-plan-hash <plan_hash> \
  --docker-socket /var/run/docker.sock \
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

GitHub Actions pouští stejné kontroly a balení při pushi a pull requestech.

## Release verze

Kanonická verze projektu je v [VERSION](VERSION).

- Git tagy používají `vMAJOR.MINOR.PATCH`.
- Unraid plugin/package verze používají `MAJOR.MINOR.PATCH`.
- Release notes jsou v [CHANGELOG.cs.md](CHANGELOG.cs.md).
- Push tagu `vMAJOR.MINOR.PATCH` nebo ruční spuštění Release workflow postaví Unraid plugin package a publikuje assets do GitHub Release.

Celá politika je ve [VERSIONING.cs.md](VERSIONING.cs.md).

## Licence

Licence zatím není vybraná. Dokud nebude přidaný licenční soubor, všechna práva si ponechává vlastník repozitáře.
