# Návrh architektury

Tento dokument popisuje cílovou architekturu Unraid AI Manageru a rozdíl mezi aktuálně hotovou preview verzí a plánovanými schopnostmi.

Anglická verze: [DESIGN.en.md](DESIGN.en.md)

## Cíl

Unraid AI Manager má umožnit AI asistentům spravovat Unraid bezpečně:

1. AI zjistí stav.
2. AI navrhne plán.
3. Uživatel uvidí diff, rizika a dopady.
4. Uživatel plán schválí lokálně.
5. Helper daemon provede pouze schválený plán.
6. Vznikne backup a audit log.
7. Změnu lze rollbacknout.

Základní princip: AI nikdy nedostává raw root shell.

## Komponenty

```text
AI klient na PC
  Claude / ChatGPT / jiný MCP klient
        |
        v
MCP server na PC
  mcp/unraid-mcp-server.mjs
        |
        v
HTTP helper na Unraidu
  unraid-ai-helper
        |
        +--> DockerMan XML templates-user
        +--> read-only Docker API
        +--> backupy
        +--> audit logy
        +--> approval token store
```

## Bezpečnostní hranice

MCP server není bezpečnostní hranice. MCP server je pohodlná integrační vrstva pro AI klienta.

Bezpečnostní hranice je helper daemon na Unraidu. Ten musí vynucovat:

- whitelist akcí;
- žádný raw shell;
- hash potvrzení plánů;
- volitelný lokální approval token;
- backup před zápisem;
- audit po zápisu;
- odmítnutí nebezpečných mountů a privilegovaných operací;
- oddělení read-only inspect akcí od write/lifecycle akcí.

## Aktuální stav v `v0.1.3`

Implementováno:

- Go CLI `unraid-ai-manager`;
- Go helper daemon `unraid-ai-helper`;
- Node MCP server `mcp/unraid-mcp-server.mjs`;
- Unraid plugin wrapper se Settings stránkou;
- DockerMan XML parser;
- AMUD planner a apply workflow;
- TZ planner a apply workflow;
- XML backup, audit a restore;
- read-only Docker API inspect;
- XML vs runtime comparison;
- read-only recreate planner;
- approval-token store.

Zatím neimplementováno:

- instalace kontejnerů z Community Applications;
- skutečný recreate/start/stop/remove kontejnerů;
- vytvoření nového kontejneru;
- Unraid shares/VM/plugin/array management;
- bezpečný Docker lifecycle proxy.

## DockerMan XML pravidla

Primární zdroj pravdy pro kontejnery spravované Unraidem jsou DockerMan XML šablony:

```text
/boot/config/plugins/dockerMan/templates-user/
```

AMUD labely se zapisují jako:

```xml
<Config Name="AMUD Enable" Target="amud.enable" Type="Label">true</Config>
<Config Name="AMUD URL" Target="amud.url" Type="Label">http://192.0.2.10:7878</Config>
<Config Name="AMUD Name" Target="amud.name" Type="Label">Radarr</Config>
<Config Name="AMUD Icon" Target="amud.icon" Type="Label">radarr</Config>
```

Nepoužíváme raw `ExtraParams --label`, protože cílem je zůstat kompatibilní s DockerMan šablonami.

## AMUD URL režimy

`local`:

```text
http://<local_host>:<host_port>
```

`cloudflare`:

```text
https://<subdomain>.<domain>
```

Použije se jen při explicitně známém routingu.

`hybrid`:

- Cloudflare pro známé routy;
- jinak lokální URL.

Host port se odvozuje z DockerMan port mappingu. Pokud `WebUI` obsahuje `[PORT:<container_port>]`, planner najde odpovídající host port.

## Approval workflow

Apply operace vyžaduje minimálně:

- uložený plán;
- `plan_hash`;
- `confirm_plan_hash`;
- backup directory;
- audit directory.

Pokud je zapnuté `require_approval_token`, je potřeba navíc krátkodobý token vytvořený lokálně na Unraidu:

```bash
unraid-ai-manager approve-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/PLAN.json \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --purpose amud \
  --ttl 15m
```

Token se ukládá jen jako SHA-256 hash a po použití se označí jako použitý.

## Community Applications plán

Cílově má nástroj umět instalovat aplikace z Community Applications tak, aby Unraid dál spravoval aktualizace a metadata.

Požadovaný flow:

1. Vyhledat aplikaci v Community Applications katalogu.
2. Načíst oficiální template.
3. Doplnit uživatelská nastavení.
4. Ukázat diff, porty, volumes, env, labels a rizika.
5. Vyžádat explicitní schválení.
6. Vytvořit DockerMan-compatible XML.
7. Vytvořit kontejner přes bezpečný lifecycle mechanismus.
8. Ověřit, že Unraid aplikaci vidí v běžném Docker/Apps workflow.

Raw Docker create bude až fallback pro custom kontejnery mimo Community Applications.

## Recreate a lifecycle plán

Recreate bude později samostatná write operace s extra schválením.

Minimální požadavky:

- inspect před akcí;
- hashovaný lifecycle plán;
- potvrzení uživatelem;
- žádné změny mimo schválený plán;
- inspect po akci;
- audit log;
- jasné hlášení, pokud container po recreate nenaběhne.

## Audit a rollback

Každý write musí uložit:

- plán;
- diff;
- původní SHA-256;
- nový SHA-256;
- backup path;
- timestamp;
- approval identitu;
- výsledek.

Backupy nesmí být ukládané do `templates-user`, aby je DockerMan/Community Applications náhodou nečetly jako šablony.

Doporučené cesty:

```text
/mnt/user/appdata/unraid-ai-manager/backups
/mnt/user/appdata/unraid-ai-manager/audit
/mnt/user/appdata/unraid-ai-manager/plans
/mnt/user/appdata/unraid-ai-manager/approvals
```
