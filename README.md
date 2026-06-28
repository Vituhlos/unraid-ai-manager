# Unraid AI Manager

Safe Unraid automation for AI assistants.

Unraid AI Manager is a local control plane for managing Unraid DockerMan templates through a strict plan → diff → approval → apply workflow. It is designed for AI/MCP clients, but the security boundary is the Unraid-side helper daemon, not the chat model.

> Current status: early preview. The latest released plugin is `v0.1.7`; the main branch may contain unreleased workflow improvements. It can inventory DockerMan XML templates, inspect Docker runtime state, plan generic dashboard/TZ/template changes, apply approved XML edits with backups and audit logs, apply approved DockerMan recreate plans, discover known app integrations without leaking full secrets, and expose those actions through an MCP server. AMUD is the first dashboard adapter. Community Applications installation and arbitrary container lifecycle management are planned, not implemented yet.

## Languages

- English: this file
- Czech: [README.cs.md](README.cs.md)
- Architecture: [DESIGN.en.md](DESIGN.en.md) / [DESIGN.md](DESIGN.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md) / [CHANGELOG.cs.md](CHANGELOG.cs.md)
- Versioning: [VERSIONING.md](VERSIONING.md) / [VERSIONING.cs.md](VERSIONING.cs.md)
- Security: [SECURITY.md](SECURITY.md) / [SECURITY.cs.md](SECURITY.cs.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md) / [CONTRIBUTING.cs.md](CONTRIBUTING.cs.md)

## What it does

- Reads Unraid DockerMan XML templates from `/boot/config/plugins/dockerMan/templates-user`.
- Exposes a capability map so AI clients can discover safe implemented/planned action modules before doing work.
- Parses ports, paths, variables, labels, WebUI, template metadata and repository information.
- Reads Docker runtime state through read-only Docker API calls.
- Compares DockerMan XML templates with live container configuration.
- Discovers known app integrations and appdata-backed API key/token locations with masked previews instead of full secret values.
- Discovery returns stable `secret_ref` identifiers for secrets. Full secret values are not exposed through MCP; future apply workflows resolve those references internally on Unraid.
- Plans dashboard configuration through provider adapters. The first adapter is AMUD through DockerMan labels:
  - `amud.enable=true`
  - `amud.url=...`
  - `amud.name=...`
  - `amud.icon=...`
- Supports dashboard URL modes:
  - `local`: `http://<local_host>:<host_port>`
  - `cloudflare`: `https://<subdomain>.<domain>`
  - `hybrid`: Cloudflare when a route is known, otherwise local
- Plans dashboard entries for DockerMan templates with an explicit `WebUI` by default.
- Can explicitly include TCP port-only templates or limit/exclude specific containers when needed.
- Helper/MCP planning filters to currently running Docker containers by default when Docker runtime access is available.
- Plans and applies `TZ` environment variable changes.
- Plans and applies approved Docker recreate operations through Unraid DockerMan `rebuild_container`.
- Can combine dashboard XML apply, DockerMan recreate and runtime verification into one approved dashboard sync workflow.
- Creates XML backups before every write.
- Requires a plan hash before applying any plan.
- Optionally requires a short-lived local approval token before apply.
- Writes audit logs.
- Restores XML templates from verified backups.
- Exposes a PC-side MCP server with whitelisted tools only.

## What it deliberately does not do yet

- It does not install Community Applications containers yet.
- It does not perform arbitrary start, stop or remove operations yet.
- It recreates containers only from an approved recreate plan through Unraid DockerMan.
- It does not accept raw shell commands from AI.
- It does not expose an unrestricted Docker socket to MCP clients.
- It does not mount the full host filesystem into an AI-controlled container.

Those capabilities are planned behind explicit policies, risk scoring and extra approval gates.

## Architecture

```text
AI client on PC
  Claude / ChatGPT / other MCP-capable client
        |
        v
PC-side MCP server
  mcp/unraid-mcp-server.mjs
        |
        v
Unraid-side helper daemon
  unraid-ai-helper
        |
        +--> DockerMan XML templates
        +--> read-only Docker API inspect
        +--> backup / audit / approval token store
```

The MCP server translates AI requests into narrow, named tools. The helper daemon enforces the policy and performs local filesystem operations on Unraid.

## Recommended installation

Install the Unraid plugin from this URL:

```text
https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg
```

In Unraid:

1. Open `Plugins`.
2. Paste the URL into `Install Plugin`.
3. Open `Settings -> Unraid AI Manager`.
4. Generate an API key.
5. Keep the helper bound to `127.0.0.1:37231` unless you have a specific reason to expose it.
6. Connect from your PC through an SSH tunnel:

```bash
ssh -L 37231:127.0.0.1:37231 root@<unraid-ip>
```

## MCP configuration

Run the MCP server on your PC. It connects to the Unraid helper over HTTP.

```powershell
$env:UNRAID_AI_HELPER_URL="http://127.0.0.1:37231"
$env:UNRAID_AI_API_KEY="<generated-api-key>"
node .\mcp\unraid-mcp-server.mjs
```

Example MCP client configuration:

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
        "UNRAID_AI_API_KEY": "<generated-api-key>"
      }
    }
  }
}
```

Available MCP tools:

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
- `unraid_plan_dashboard_integrations`
- `unraid_plan_amud`
- `unraid_apply_amud`
- `unraid_plan_tz`
- `unraid_apply_tz`
- `unraid_plan_recreate`
- `unraid_apply_recreate`
- `unraid_restore_xml`

Apply tools require `confirm_plan_hash`. When approval tokens are enabled, they also require `approval_token`.

## Safe dashboard workflow

1. Ask the AI to create a plan, not to apply it.
2. Review the proposed provider, adapter, URLs, target changes, risks and XML diff.
3. Confirm the exact `plan_hash`.
4. Create a short-lived local approval token on Unraid:

```bash
unraid-ai-manager approve-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/PLAN.json \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --purpose dashboard \
  --ttl 15m
```

5. Let the AI call the apply tool with the plan hash and token.
6. Verify the audit log and resulting XML.
7. Prefer the dashboard sync workflow when the change should also be pushed into live containers. It plans XML changes, recreate operations and runtime verification under one plan hash.

## Manual CLI examples

Inventory:

```bash
unraid-ai-manager inventory \
  --templates /boot/config/plugins/dockerMan/templates-user
```

Discover known integrations and masked API key/token locations:

```bash
unraid-ai-manager discover-integrations \
  --templates /boot/config/plugins/dockerMan/templates-user
```

The output includes masked previews and `secret_ref` values. Treat `secret_ref` as a capability reference, not as a secret value; it is only useful to approved local workflows.

Plan dashboard configuration through the AMUD adapter:

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

Apply an approved dashboard plan:

```bash
unraid-ai-manager apply-dashboard-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/dashboard-plan.json \
  --confirm-plan-hash <plan_hash> \
  --backup-dir /mnt/user/appdata/unraid-ai-manager/backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit
```

Plan dashboard configuration and runtime propagation as one sync workflow:

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

Apply an approved dashboard sync plan:

```bash
unraid-ai-manager apply-dashboard-sync-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/dashboard-sync-plan.json \
  --confirm-plan-hash <sync_plan_hash> \
  --backup-dir /mnt/user/appdata/unraid-ai-manager/backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit \
  --docker-socket /var/run/docker.sock
```

`plan-amud` and `apply-amud-plan` remain as compatibility shortcuts for the AMUD label adapter, but new workflows should prefer the generic dashboard commands.

Apply an approved recreate plan:

```bash
unraid-ai-manager apply-recreate-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/recreate-plan.json \
  --confirm-plan-hash <plan_hash> \
  --docker-socket /var/run/docker.sock \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit
```

Restore an XML backup:

```bash
unraid-ai-manager restore-xml-backup \
  --backup /mnt/user/appdata/unraid-ai-manager/backups/my-container.xml \
  --target /boot/config/plugins/dockerMan/templates-user/my-container.xml \
  --confirm-backup-sha256 <backup_sha256> \
  --pre-restore-backup-dir /mnt/user/appdata/unraid-ai-manager/restore-backups \
  --audit-dir /mnt/user/appdata/unraid-ai-manager/audit
```

## Development

The repository uses Go for the production helper/CLI and keeps the older Python prototype as reference material.

Portable Go can live in:

```text
.tools/go/go1.26.4/go/bin/go.exe
```

Run tests:

```powershell
& ".\.tools\go\go1.26.4\go\bin\go.exe" test ./...
node --check .\mcp\unraid-mcp-server.mjs
```

Build binaries:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Build Unraid plugin artifacts:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-unraid-plugin.ps1
```

Outputs are written to `dist/`.

GitHub Actions also runs the same checks and packaging on pushes and pull requests.

## Release version

The canonical project version is stored in [VERSION](VERSION).

- Git tags use `vMAJOR.MINOR.PATCH`.
- Unraid plugin/package versions use `MAJOR.MINOR.PATCH`.
- Release notes are maintained in [CHANGELOG.md](CHANGELOG.md).
- Pushing a `vMAJOR.MINOR.PATCH` tag or manually running the Release workflow builds the Unraid plugin package and publishes GitHub Release assets.

See [VERSIONING.md](VERSIONING.md) for the full policy.

## License

No license has been selected yet. Until a license is added, all rights are reserved by the repository owner.
