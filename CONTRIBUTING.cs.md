# Přispívání

Projekt je zatím v rané fázi a bezpečnostně citlivý. Změny mají být malé, dobře zkontrolovatelné a explicitní v dopadu na uživatele.

## Vývojová pravidla

- Produkční CLI/helper kód piš primárně v Go.
- MCP server drž úzký: pouze pojmenované tools, žádný raw shell passthrough.
- Nepřidávej write operace bez plánu, diffu, potvrzení a auditu.
- Nepřidávej široký přístup k Docker socketu nebo filesystemu bez aktualizace bezpečnostního návrhu.
- Při změně user-facing chování drž anglickou a českou dokumentaci synchronní.

## Commit messages

Používej stručné commit messages ve stylu Conventional Commits.

Doporučený formát:

```text
type(scope): krátký popis

EN: User-facing explanation in English.
CS: Uživatelské vysvětlení česky.
```

Příklady:

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

Povolené typy:

- `feat`: uživatelsky viditelná funkce
- `fix`: oprava chyby
- `docs`: pouze dokumentace
- `security`: bezpečnostní hardening nebo oprava zranitelnosti
- `refactor`: interní přestavba bez změny chování
- `test`: testy
- `build`: build, packaging nebo release automatizace
- `chore`: údržba repozitáře

## Pravidla changelogu

Changelog se řídí [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).

- Pro každou user-facing změnu aktualizuj `CHANGELOG.md` i `CHANGELOG.cs.md`.
- Nahoře drž sekci `Unreleased`.
- Používej ISO datum: `YYYY-MM-DD`.
- Změny seskupuj pod `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` a `Security`.
- Nedělej z changelogu dump git logu.

## Pull request checklist

- [ ] Testy procházejí.
- [ ] Změny MCP tool schémat jsou zdokumentované.
- [ ] Změny helper endpointů jsou zdokumentované.
- [ ] Bezpečnostní dopady jsou popsané.
- [ ] `CHANGELOG.md` a `CHANGELOG.cs.md` jsou aktualizované, pokud je potřeba.
- [ ] `VERSION` se mění jen při přípravě release.
