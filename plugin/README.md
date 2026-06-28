# Unraid plugin packaging

This folder contains the Unraid plugin wrapper for the Unraid AI helper.

The plugin installs:

- `/usr/local/bin/unraid-ai-helper`
- `/usr/local/bin/unraid-ai-manager`
- `/etc/rc.d/rc.unraid-ai-manager`
- `/usr/local/emhttp/plugins/unraid-ai-manager/*`

Persistent config lives in:

```text
/boot/config/plugins/unraid-ai-manager/unraid-ai-manager.cfg
```

Backups, plans, approvals and audit logs should live in appdata:

```text
/mnt/user/appdata/unraid-ai-manager/
```

Build:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1 `
  -Version 2026.06.28 `
  -PackageUrl "https://github.com/Vituhlos/unraid-ai-manager/releases/download/v0.1.0" `
  -PluginUrl "https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg"
```

Outputs:

```text
dist/unraid-ai-manager-<version>-x86_64-1.txz
dist/unraid-ai-manager.plg
```
