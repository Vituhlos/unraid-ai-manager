# Návrh: Unraid AI Manager

## Cíl

Postavit Unraid-native control-plane, který AI umožní spravovat Docker kontejnery, Community Applications, AMUD labely a později širší Unraid funkce, ale bez toho, aby AI dostala libovolný root shell.

Primární implementační jazyk pro core/daemon je Go. Python část v repu slouží jako rychlý referenční prototyp pro read-only fázi.

Požadovaný UX:

1. Uživatel řekne AI, co chce udělat.
2. MCP server zjistí stav Unraidu.
3. Vytvoří se plán s diffem, riziky a dopady.
4. Uživatel plán lokálně schválí.
5. Lokální daemon provede jen přesně schválený plán.
6. Výsledek se ověří, zaloguje a lze ho rollbacknout.

## Architektura

```text
Claude / ChatGPT / jiná AI
        |
        v
MCP server
- intent -> bezpečné nástroje
- inventory
- planning
- vysvětlení rizik
- request approval
        |
        v
Unraid Manager daemon
- policy engine
- plánovací hash
- lokální approval token
- backup
- apply
- audit
- rollback
        |
        +--> Unraid GraphQL API
        +--> DockerMan XML templates-user
        +--> Community Applications katalog/templates
        +--> Docker API proxy pro inspect/lifecycle
```

MCP server není bezpečnostní hranice. Bezpečnostní hranice je lokální daemon s whitelistem akcí a policy enginem.

Aktuální implementace už obsahuje první verzi HTTP helper daemonu:

- `unraid-ai-helper`,
- bind defaultně na `127.0.0.1:37231`,
- volitelný `X-Unraid-AI-Key` / Bearer token,
- read-only inventory/inspect/compare endpointy,
- plan endpointy pro AMUD, TZ a recreate,
- apply endpointy pro AMUD a TZ s hash potvrzením,
- restore endpoint pro XML rollback.
- volitelně vynucuje lokální approval token pro apply endpointy.

Aktuální implementace také obsahuje první MCP stdio server:

- `mcp/unraid-mcp-server.mjs`,
- běží na PC,
- volá HTTP helper přes `UNRAID_AI_HELPER_URL`,
- autentizuje se přes `UNRAID_AI_API_KEY`,
- vystavuje pouze pojmenované nástroje, žádný raw shell.

Plugin packaging:

- plugin wrapper instaluje helper/CLI binárky,
- přidává `/etc/rc.d/rc.unraid-ai-manager`,
- přidává Settings stránku,
- ukládá persistent config do `/boot/config/plugins/unraid-ai-manager`,
- používá appdata pro backupy, plány, approvals a audit,
- pro plnohodnotný Plugin Manager install musí být `.plg` a `.txz` publikované přes stabilní URL.

Approval token model:

- plán má `plan_hash`,
- uživatel lokálně na Unraidu spustí `approve-plan`,
- token se uloží jen jako SHA-256 hash do approval recordu,
- token má expiraci,
- po použití se označí jako použitý,
- apply endpoint vyžaduje `confirm_plan_hash` i `approval_token`.

## Proč Unraid-native instalace

Kontejnery z Community Applications se nesmí defaultně vytvářet jen přes raw Docker API. Pokud aplikace existuje v Community Applications, nástroj musí použít Community Apps/DockerMan kompatibilní template flow, aby Unraid dál viděl aplikaci jako běžně spravovanou:

- Docker tab,
- Apps / Installed Apps,
- Action Center,
- Previous Apps,
- update/reinstall flow,
- metadata jako `TemplateURL`, `Support`, `Project`, `Icon`, `Category`.

Raw Docker create flow bude až výslovný fallback pro custom kontejnery mimo Community Applications.

## Fáze

### Fáze 0: návrh a read-only inventory

- načíst DockerMan XML z `templates-user`,
- parsovat `Config Type="Port|Path|Variable|Label"`,
- vypsat WebUI, Repository, Network, TemplateURL,
- spočítat základní rizika,
- žádné zápisy.

### Fáze 1: AMUD planner

- detekovat webové kontejnery,
- navrhnout `amud.enable`,
- navrhnout `amud.url`,
- navrhnout `amud.name`,
- navrhnout `amud.icon`,
- podporovat URL režimy `local`, `cloudflare`, `hybrid`,
- ukázat plán a diff.
- exportovat hashovaný plán jako JSON.

### Fáze 2: XML writer bez Docker lifecycle

- načíst exportovaný plán,
- ověřit hash plánu,
- validovat plán,
- hash původního XML,
- backup mimo `templates-user`,
- atomický zápis XML,
- audit log,
- rollback.

Aktuální první executor implementuje pouze AMUD label patch a vyžaduje `--confirm-plan-hash`. Lokální UI approval token bude další vrstva nad stejným principem.

Rollback je implementovaný jako obecný restore XML backup:

- vyžaduje přesný SHA-256 hash backup souboru,
- validuje backup i cílový XML jako `<Container>`,
- před restore vytvoří pre-restore backup aktuálního cíle,
- zapisuje audit log,
- odmítá backup/audit adresáře uvnitř template adresáře.

### Fáze 3: Docker lifecycle

- read-only Docker inspect parser,
- porovnání DockerMan XML vs runtime stav,
- recreate/start/stop/restart jen přes schválený plán,
- Docker inspect před i po,
- žádný raw shell.

Aktuálně je implementovaná read-only část: načtení uloženého `docker inspect` JSON a porovnání portů, AMUD labelů, env proměnných, image a network proti DockerMan XML.

Také je implementovaný read-only `docker-recreate` planner. Ten vytvoří hashovaný plán pro kontejnery, u kterých runtime stav neodpovídá šablonám. Zatím nic nerecreatuje.

### Fáze 4: Community Apps install

- vyhledat aplikaci v Community Applications,
- načíst template,
- ukázat rizika,
- doplnit hodnoty,
- vytvořit DockerMan-compatible XML,
- vytvořit container,
- ověřit, že ho Unraid vidí.

### Fáze 5: širší Unraid

- shares,
- VMs,
- plugins,
- notifications,
- array read-only,
- později vybrané array operace s extra potvrzením.

## Schvalování

Schválení nesmí být jen text v AI chatu. Pro nebezpečné akce bude flow:

1. MCP vytvoří plán.
2. Daemon uloží plán a spočítá hash.
3. Uživatel otevře lokální UI/CLI na Unraidu.
4. Uživatel klikne/potvrdí `Apply`.
5. Daemon vydá krátkodobý approval token.
6. Apply endpoint provede jen plán se stejným hashem.

Tím se snižuje riziko prompt injection a omylů v konverzaci.

## Policy model

Akce jsou whitelisted. Co není explicitně povolené, je zakázané.

Povolené v prvních write fázích:

- přidat/upravit `amud.*` labely,
- přidat/upravit `TZ`,
- změnit AMUD URL podle schváleného URL režimu,
- backup/restore XML,
- recreate/start/stop/restart existujícího kontejneru po potvrzení,
- instalace Community Apps kontejneru po potvrzení.

Zakázané nebo extra potvrzení:

- raw shell,
- arbitrary XPath/XML editace,
- `Privileged=true`,
- mount `/`, `/boot`, `/etc`, `/usr`, `/var/run/docker.sock`,
- mount celého `/mnt` nebo `/mnt/user` bez extra potvrzení,
- host networking,
- devices,
- capabilities,
- plugin install,
- mazání appdata,
- disk format,
- změny disk assignmentu,
- array stop/start bez samostatné politiky.

## Docker socket

Read-only bind Docker socketu není reálná bezpečnostní hranice. Produkční návrh:

- hlavní app nemá přímý neomezený socket,
- oddělený Docker API proxy povolí jen konkrétní operace,
- lifecycle operace budou mapované na whitelist,
- proxy nebude vystavené do LAN/Internetu.

Do doby, než existuje proxy, CLI přijímá uložený `docker inspect` JSON snapshot. To umožňuje vývoj a preflight porovnání bez přístupu k socketu.

## DockerMan XML pravidla

Unraid DockerMan v aktuálních šablonách používá hlavně:

```xml
<Container version="2">
  <Name>...</Name>
  <Repository>...</Repository>
  <Network>bridge</Network>
  <WebUI>http://[IP]:[PORT:9696]/</WebUI>
  <TemplateURL>...</TemplateURL>
  <Config Name="WebUI" Target="9696" Mode="tcp" Type="Port">9696</Config>
  <Config Name="Appdata" Target="/config" Mode="rw" Type="Path">/mnt/user/appdata/app</Config>
  <Config Name="TZ" Target="TZ" Type="Variable">Europe/Prague</Config>
  <Config Name="AMUD Enable" Target="amud.enable" Type="Label">true</Config>
</Container>
```

Pro labely budeme používat `Config Type="Label"`, ne `ExtraParams --label`.

## URL pravidla pro AMUD

`local`:

```text
http://<local_host>:<host_port>
```

`cloudflare`:

```text
https://<subdomain>.<domain>
```

Jen pokud existuje explicitní mapping.

`hybrid`:

- Cloudflare, pokud existuje mapping,
- jinak local.

Web port se bere primárně z `[PORT:<container_port>]` ve `WebUI`, protože Unraid používá container port a mapuje ho na host port z `Config Type="Port"`.

## Audit a rollback

Každý apply musí uložit:

- plan JSON,
- diff,
- původní SHA-256,
- nový SHA-256,
- timestamp,
- uživatele/schvalovací identitu,
- výsledek,
- log lifecycle operací.

Backupy XML nesmí být ukládané do `templates-user` podadresáře, protože DockerMan/CA může číst XML i z nečekaných míst pod touto cestou. Doporučené umístění:

```text
/mnt/user/appdata/unraid-ai-manager/backups
/mnt/user/appdata/unraid-ai-manager/audit
```
