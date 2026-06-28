# Unraid AI Manager

Bezpečný návrh a první read-only prototyp nástroje pro správu Unraidu přes AI/MCP.

Aktuální stav projektu:

- čte Unraid DockerMan XML šablony,
- rozpozná porty, proměnné, cesty a labely,
- umí vypsat inventory,
- umí navrhnout AMUD labely bez zápisu do XML,
- zatím neumí nic aplikovat.

Produkční cíl je Unraid-native manager: AI navrhne plán, člověk ho lokálně schválí a lokální daemon ho teprve potom provede s backupem, auditem a rollbackem.

## Lokální Go toolchain

Go je pro vývoj nainstalované portable do workspace:

```text
.tools/go/go1.26.4/go/bin/go.exe
```

Adresář `.tools/` je v `.gitignore`, takže se runtime nebude commitovat.

## Lokální spuštění Go CLI

Z workspace:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager inventory --templates "C:\path\to\templates"
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager plan-amud --templates "C:\path\to\templates" --local-host 192.0.2.10
```

Preview XML diffu bez zápisu:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager plan-amud `
  --templates "C:\path\to\templates" `
  --local-host 192.0.2.10 `
  --diff
```

Export hashovaného plánu:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager plan-amud `
  --templates "C:\path\to\templates" `
  --local-host 192.0.2.10 `
  --diff `
  --out ".\amud-plan.json"
```

Aplikace exportovaného AMUD plánu:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager apply-amud-plan `
  --plan ".\amud-plan.json" `
  --confirm-plan-hash "<HASH_Z_PREVIEW>" `
  --backup-dir ".\.local\backups" `
  --audit-dir ".\.local\audit"
```

Rollback/obnova XML ze zálohy:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager restore-xml-backup `
  --backup "C:\path\to\backup.xml" `
  --target "C:\path\to\template.xml" `
  --confirm-backup-sha256 "<SHA256_ZALOHY>" `
  --pre-restore-backup-dir ".\.local\restore-backups" `
  --audit-dir ".\.local\audit"
```

Restore před přepsáním cílového XML vždy zazálohuje aktuální cílový soubor do `--pre-restore-backup-dir`.

Později na Unraidu budou vhodnější adresáře například:

```text
/mnt/user/appdata/unraid-ai-manager/backups
/mnt/user/appdata/unraid-ai-manager/restore-backups
/mnt/user/appdata/unraid-ai-manager/audit
```

Hybrid Cloudflare mapping:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager plan-amud `
  --templates "C:\path\to\templates" `
  --url-mode hybrid `
  --local-host 192.0.2.10 `
  --cloudflare-domain example.com `
  --route Seerr=seerr `
  --route prowlarr=prowlarr
```

Testy:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" test ./...
```

Build binárek:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Výstupy:

```text
dist/unraid-ai-helper-linux-amd64
dist/unraid-ai-helper-linux-amd64.sha256
dist/unraid-ai-manager-linux-amd64
dist/unraid-ai-manager-linux-amd64.sha256
dist/unraid-ai-helper-windows-amd64.exe
dist/unraid-ai-helper-windows-amd64.exe.sha256
dist/unraid-ai-manager-windows-amd64.exe
dist/unraid-ai-manager-windows-amd64.exe.sha256
```

Linux binár `unraid-ai-helper-linux-amd64` je určený pro Unraid jako bezpečný lokální agent. `unraid-ai-manager` je ruční CLI nástroj.

## Unraid plugin

Unraid část jde zabalit jako nativní plugin. Plugin je wrapper kolem helper daemonu; MCP server pořád běží na PC.

Build plugin artefaktů:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1 `
  -Version 2026.06.28 `
  -PackageUrl "https://github.com/Vituhlos/unraid-ai-manager/releases/download/v0.1.0" `
  -PluginUrl "https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg"
```

Výstupy:

```text
dist/unraid-ai-manager.plg
dist/unraid-ai-manager-2026.06.28-x86_64-1.txz
dist/unraid-ai-manager-2026.06.28-x86_64-1.txz.sha256
```

Plugin Manager install vyžaduje, aby `.plg` i `.txz` byly dostupné přes URL. Pro reálný GUI install je tedy potřeba nahrát je například do GitHub Release a vygenerovat `.plg` s tímto URL.

Lokální ruční test na Unraidu bez Plugin Manageru:

```bash
installpkg /boot/unraid-ai-manager-2026.06.28-x86_64-1.txz
mkdir -p /boot/config/plugins/unraid-ai-manager
cp /usr/local/emhttp/plugins/unraid-ai-manager/README.md /boot/config/plugins/unraid-ai-manager/README.installed
chmod +x /etc/rc.d/rc.unraid-ai-manager /usr/local/bin/unraid-ai-helper /usr/local/bin/unraid-ai-manager
/etc/rc.d/rc.unraid-ai-manager start
```

Po plugin instalaci bude stránka v Settings -> Unraid AI Manager.

## Unraid helper daemon

Cílový model:

```text
Claude/ChatGPT na PC
  -> MCP server na PC
    -> HTTP helper na Unraidu
      -> DockerMan XML / Docker inspect / backup / audit
```

Lokální start na Unraidu:

```bash
chmod +x ./unraid-ai-helper-linux-amd64

UNRAID_AI_API_KEY="zvol-si-dlouhy-token" ./unraid-ai-helper-linux-amd64 \
  --listen 127.0.0.1:37231 \
  --templates /boot/config/plugins/dockerMan/templates-user \
  --backup-dir /mnt/user/appdata/unraid-ai-manager/backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit \
  --plans-dir /mnt/user/appdata/unraid-ai-manager/plans \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --docker-socket /var/run/docker.sock \
  --local-host 192.0.2.10 \
  --require-approval-token=true
```

Defaultně doporučuju `127.0.0.1`. Pro přístup z PC se dá použít SSH tunnel:

```bash
ssh -L 37231:127.0.0.1:37231 root@192.0.2.10
```

Pak z PC:

```powershell
Invoke-RestMethod `
  -Uri "http://127.0.0.1:37231/v1/inventory" `
  -Headers @{ "X-Unraid-AI-Key" = "zvol-si-dlouhy-token" }
```

Hlavní endpointy:

```text
GET  /v1/health
GET  /v1/inventory
GET  /v1/docker/inspect
GET  /v1/runtime/compare
POST /v1/plan/amud
POST /v1/plan/tz
POST /v1/plan/recreate
POST /v1/apply/amud
POST /v1/apply/tz
POST /v1/restore/xml
```

Apply endpointy pořád vyžadují potvrzovací hash plánu.

Pokud je zapnuté `--require-approval-token=true`, apply endpointy navíc vyžadují lokální approval token. Token vytvoříš přímo na Unraidu:

```bash
./unraid-ai-manager-linux-amd64 approve-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/PLAN.json \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --purpose amud \
  --ttl 15m
```

Teprve token z tohoto příkazu vložíš do AI/MCP apply volání jako `approval_token`.

Bezpečný reálný AMUD flow:

```text
1. AI zavolá unraid_plan_amud s include_diffs/save_plan.
2. Ty zkontroluješ diff a plan_hash.
3. Na Unraidu spustíš approve-plan pro uložený plán.
4. AI zavolá unraid_apply_amud s confirm_plan_hash a approval_token.
5. Helper udělá backup, XML patch a audit log.
```

## MCP server na PC

MCP server je v:

```text
mcp/unraid-mcp-server.mjs
```

Spouští se na PC a volá Unraid helper přes HTTP:

```powershell
$env:UNRAID_AI_HELPER_URL="http://127.0.0.1:37231"
$env:UNRAID_AI_API_KEY="zvol-si-dlouhy-token"
node .\mcp\unraid-mcp-server.mjs
```

Příklad konfigurace pro MCP klienta:

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
        "UNRAID_AI_API_KEY": "zvol-si-dlouhy-token"
      }
    }
  }
}
```

Aktuální MCP tools:

```text
unraid_health
unraid_inventory
unraid_docker_inspect
unraid_compare_runtime
unraid_plan_amud
unraid_apply_amud
unraid_plan_tz
unraid_apply_tz
unraid_plan_recreate
unraid_restore_xml
```

Apply tools nejsou “volný shell”; jen posílají schválený plán/hash do helperu.
Při zapnutém approval režimu musí navíc dostat `approval_token`, který MCP server neumí sám vytvořit.

## Docker inspect read-only workflow

Nástroj umí načíst uložený JSON z `docker inspect`. Zatím tím obcházíme přímý přístup k Docker socketu; později to nahradí bezpečný Docker API proxy.

Na Unraidu lze ručně získat read-only snapshot například:

```bash
docker inspect $(docker ps -aq) > /mnt/user/appdata/unraid-ai-manager/inspect.json
```

Na vývojovém stroji:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager inspect-json `
  --inspect "C:\path\to\inspect.json"
```

Na Unraidu lze později číst runtime stav přímo přes Docker socket:

```bash
./unraid-ai-manager inspect-docker --docker-socket /var/run/docker.sock
```

Tento příkaz používá jen Docker API `GET` volání. Pro produkční nasazení pořád platí, že lepší cílový model je úzký Docker API proxy, ne obecný socket mount pro MCP.

Porovnání DockerMan XML šablon proti runtime inspect snapshotu:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" run ./cmd/unraid-ai-manager compare-runtime `
  --templates "C:\path\to\templates" `
  --inspect "C:\path\to\inspect.json"
```

Porovnání přímo proti Docker socketu:

```bash
./unraid-ai-manager compare-runtime \
  --templates /boot/config/plugins/dockerMan/templates-user \
  --docker-socket /var/run/docker.sock
```

Read-only recreate plán:

```bash
./unraid-ai-manager plan-recreate \
  --templates /boot/config/plugins/dockerMan/templates-user \
  --docker-socket /var/run/docker.sock \
  --out /mnt/user/appdata/unraid-ai-manager/plans/recreate-plan.json
```

## Lokální spuštění Python referenčního prototypu

Z workspace:

```powershell
$env:PYTHONPATH="src"
python -m unraid_ai_manager.cli inventory --templates "C:\path\to\templates"
python -m unraid_ai_manager.cli plan-amud --templates "C:\path\to\templates" --local-host 192.0.2.10
```

JSON výstup:

```powershell
$env:PYTHONPATH="src"
python -m unraid_ai_manager.cli inventory --templates "C:\path\to\templates" --json
python -m unraid_ai_manager.cli plan-amud --templates "C:\path\to\templates" --local-host 192.0.2.10 --json
```

Hybrid Cloudflare mapping:

```powershell
$env:PYTHONPATH="src"
python -m unraid_ai_manager.cli plan-amud `
  --templates "C:\path\to\templates" `
  --url-mode hybrid `
  --local-host 192.0.2.10 `
  --cloudflare-domain example.com `
  --route Seerr=seerr `
  --route prowlarr=prowlarr
```

## Bezpečnostní pravidlo MVP

Planner příkazy jsou read-only vůči Unraid šablonám. `apply-amud-plan` už XML mění, ale jen když dostane:

- exportovaný plán,
- přesný `--confirm-plan-hash`,
- `--backup-dir`,
- `--audit-dir`.

Bez toho odmítne běžet.

`restore-xml-backup` obnovuje XML ze zálohy, ale jen když dostane přesný SHA256 hash zálohy. Před restore ještě uloží aktuální target XML jako pre-restore backup.

Planner ani apply v této fázi:

- nesahá na Docker socket,
- nerecreatuje kontejnery,
- neinstaluje Community Apps,
- nepřijímá raw shell příkazy.

Příkaz `--out` zapisuje pouze plánový JSON tam, kam mu explicitně řekneš. Nemění DockerMan šablony.

Aktuální executor umí pouze AMUD label XML patch. Docker lifecycle a Community Apps install zatím nejsou implementované.

Docker inspect workflow je read-only. Nepotřebuje Docker socket a nic nerecreatuje.

`plan-recreate` je taky read-only. Pouze řekne, které kontejnery pravděpodobně potřebují recreate, protože runtime stav neodpovídá DockerMan XML.
