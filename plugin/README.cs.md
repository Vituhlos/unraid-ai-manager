# Balení Unraid pluginu

Tato složka obsahuje Unraid plugin wrapper pro Unraid AI Manager.

Anglická verze: [README.md](README.md)

## Instalované soubory

Plugin instaluje:

- `/usr/local/bin/unraid-ai-helper`
- `/usr/local/bin/unraid-ai-manager`
- `/etc/rc.d/rc.unraid-ai-manager`
- `/usr/local/emhttp/plugins/unraid-ai-manager/*`

Persistentní konfigurace:

```text
/boot/config/plugins/unraid-ai-manager/unraid-ai-manager.cfg
```

Runtime data:

```text
/mnt/user/appdata/unraid-ai-manager/
```

V appdata složce jsou backupy, plány, approval záznamy a audit logy.

## Build

Packaging script defaultně čte verzi z kořenového souboru `VERSION`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1
```

Výstupy:

```text
dist/unraid-ai-manager-<version>-x86_64-1.txz
dist/unraid-ai-manager-<version>-x86_64-1.txz.sha256
dist/unraid-ai-manager.plg
```

Pro konkrétní release tag:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1 `
  -Version 0.1.1 `
  -ReleaseTag v0.1.1
```
