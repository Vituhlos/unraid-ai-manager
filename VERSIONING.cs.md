# Politika verzování

Unraid AI Manager se řídí [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

## Kanonická verze

Kanonická verze projektu je uložená v:

```text
VERSION
```

Hodnota musí být SemVer verze bez úvodního `v`, například:

```text
0.1.1
```

## Tagy a artefakty

- GitHub release tagy používají `vMAJOR.MINOR.PATCH`, například `v0.1.1`.
- Unraid plugin verze používají `MAJOR.MINOR.PATCH`, například `0.1.1`.
- Unraid package názvy používají `unraid-ai-manager-MAJOR.MINOR.PATCH-x86_64-1.txz`.
- Release notes se berou z [CHANGELOG.cs.md](CHANGELOG.cs.md) / [CHANGELOG.md](CHANGELOG.md).

## Pravidla bumpování ve fázi `0.y.z`

Projekt je zatím v rané vývojové fázi `0.y.z`. API, MCP tools a helper endpointy se ještě mohou měnit.

- `PATCH` bump: opravy dokumentace, packaging fixy, interní cleanup, malé zpětně kompatibilní opravy.
- `MINOR` bump: nová uživatelsky viditelná schopnost, nový MCP tool, nový helper endpoint, nový apply workflow, nová schopnost plugin UI.
- `MAJOR` bump: rezervováno pro `1.0.0` nebo jasně záměrný compatibility reset.

## Pravidla bumpování po `1.0.0`

- `PATCH`: zpětně kompatibilní bugfixy.
- `MINOR`: zpětně kompatibilní funkcionalita.
- `MAJOR`: zpětně nekompatibilní změny veřejného API, MCP tools nebo helper endpointů.

## Neměnnost releasů

Publikované release artefakty se nemají upravovat na místě. Pokud je artefakt špatně, vytvoří se nová verze.

Výjimka: GitHub release notes lze upravit kvůli překlepům nebo doplnění odkazů, ale ne kvůli zakrytí funkčních změn.

## Pre-release verze

Později lze používat pre-release tagy:

```text
v0.2.0-alpha.1
v0.2.0-rc.1
```

Unraid plugin releasy by měly preferovat stabilní tagy, pokud release není jasně označený jako experimentální.
