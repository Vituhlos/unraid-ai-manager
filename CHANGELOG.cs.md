# Changelog

V tomto souboru jsou dokumentované všechny podstatné změny projektu.

Formát vychází z [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) a projekt se řídí [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

> Názvy sekcí `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` a `Security` zůstávají anglicky kvůli kompatibilitě s běžnou strukturou Keep a Changelog. Popisy změn jsou česky.

## [Unreleased]

## [0.1.2] - 2026-06-28

### Fixed

- Opravena prázdná stránka nastavení Unraid pluginu přidáním povinného `.page` oddělovače obsahu a explicitního includu PHP UI souboru.
- Sníženo riziko kolize PHP helper funkcí s Unraid/Dynamix funkcemi prefixem `uaim_`.

### Added

- Přidán validační skript pluginu, který před publikací kontroluje `.page` strukturu, konzistenci verzí a release URL ve vygenerovaném `.plg`.

## [0.1.1] - 2026-06-28

### Added

- Anglický a český README.
- Anglický a český changelog podle Keep a Changelog 1.1.0.
- Anglická a česká politika verzování podle Semantic Versioning 2.0.0.
- Anglická a česká bezpečnostní politika.
- Anglický a český contributing guide s pravidly pro commit messages.
- Release checklist pro GitHub a Unraid plugin releasy.
- Kanonický soubor `VERSION`.
- Návrhová dokumentace pro budoucí instalaci kontejnerů z Community Applications.
- Release governance dokumentace pro commit messages, verzování a release checklist.

### Changed

- Packaging script pro Unraid plugin nově defaultně používá kanonickou SemVer verzi ze souboru `VERSION`.
- GitHub release tagy jsou sjednocené na `vMAJOR.MINOR.PATCH`.
- Unraid plugin/package verze jsou sjednocené na `MAJOR.MINOR.PATCH`.
- README je přepsané tak, aby přesně popisovalo aktuální helper, MCP a approval-token workflow.
- Dokumentace projektu je nově udržovaná v angličtině i češtině.

## [0.1.0] - 2026-06-28

### Added

- První Go CLI pro čtení Unraid DockerMan XML šablon.
- První helper daemon běžící na Unraidu s HTTP endpointy pro health, inventory, Docker inspect, runtime compare, AMUD planning, TZ planning, recreate planning, AMUD apply, TZ apply a XML restore.
- První MCP server běžící na PC s whitelistovanými Unraid nástroji.
- Podpora XML backupů, audit logů a rollbacku.
- AMUD label workflow potvrzovaný hashem plánu.
- TZ env workflow potvrzovaný hashem plánu.
- Volitelný krátkodobý approval-token model pro apply operace.
- Read-only Docker API client pro inventory a inspect kontejnerů.
- Runtime porovnání DockerMan XML šablon proti živé Docker konfiguraci.
- Read-only recreate planner.
- Unraid plugin packaging se Settings stránkou, rc skriptem a appdata konfigurací.
- Build skripty pro Windows a Linux binárky.
- Python referenční prototyp pro rané XML planning experimenty.

### Security

- MCP nástroje nepřijímají raw shell příkazy.
- Apply operace vyžadují přesný hash plánu.
- XML restore vyžaduje přesný SHA-256 hash zálohy.
- Helper se defaultně binduje na `127.0.0.1:37231`.
- Helper podporuje autentizaci přes API key.

[Unreleased]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Vituhlos/unraid-ai-manager/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Vituhlos/unraid-ai-manager/releases/tag/v0.1.0
