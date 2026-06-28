# Architecture Design

This document describes the target architecture of Unraid AI Manager and distinguishes the current preview implementation from planned capabilities.

Czech version: [DESIGN.md](DESIGN.md)

## Goal

Unraid AI Manager should allow AI assistants to manage Unraid safely:

1. The AI reads state.
2. The AI proposes a plan.
3. The user reviews the diff, risks and impact.
4. The user approves the plan locally.
5. The helper daemon applies only the approved plan.
6. Backups and audit logs are created.
7. The change can be rolled back.

Core principle: the AI never receives a raw root shell.

## Components

```text
AI client on PC
  Claude / ChatGPT / other MCP client
        |
        v
PC-side MCP server
  mcp/unraid-mcp-server.mjs
        |
        v
HTTP helper on Unraid
  unraid-ai-helper
        |
        +--> DockerMan XML templates-user
        +--> read-only Docker API
        +--> backups
        +--> audit logs
        +--> approval token store
```

## Security boundary

The MCP server is not the security boundary. It is an integration layer for the AI client.

The Unraid-side helper daemon is the enforcement point. It must enforce:

- action whitelist;
- no raw shell execution;
- plan hash confirmation;
- optional local approval token;
- backup before write;
- audit after write;
- rejection of dangerous mounts and privileged operations;
- separation between read-only inspect actions and write/lifecycle actions.

## Current status in `v0.1.1`

Implemented:

- Go CLI `unraid-ai-manager`;
- Go helper daemon `unraid-ai-helper`;
- Node MCP server `mcp/unraid-mcp-server.mjs`;
- Unraid plugin wrapper with Settings page;
- DockerMan XML parser;
- AMUD planner and apply workflow;
- TZ planner and apply workflow;
- XML backup, audit and restore;
- read-only Docker API inspect;
- XML vs runtime comparison;
- read-only recreate planner;
- approval-token store.

Not implemented yet:

- Community Applications container installation;
- actual recreate/start/stop/remove lifecycle operations;
- new container creation;
- Unraid shares/VM/plugin/array management;
- safe Docker lifecycle proxy.

## DockerMan XML rules

The primary source of truth for Unraid-managed containers is DockerMan XML:

```text
/boot/config/plugins/dockerMan/templates-user/
```

AMUD labels are written as:

```xml
<Config Name="AMUD Enable" Target="amud.enable" Type="Label">true</Config>
<Config Name="AMUD URL" Target="amud.url" Type="Label">http://192.0.2.10:7878</Config>
<Config Name="AMUD Name" Target="amud.name" Type="Label">Radarr</Config>
<Config Name="AMUD Icon" Target="amud.icon" Type="Label">radarr</Config>
```

The tool does not use raw `ExtraParams --label`, because the goal is to stay compatible with DockerMan templates.

## AMUD URL modes

`local`:

```text
http://<local_host>:<host_port>
```

`cloudflare`:

```text
https://<subdomain>.<domain>
```

Used only when an explicit route is known.

`hybrid`:

- Cloudflare for known routes;
- local URL otherwise.

The host port is derived from DockerMan port mappings. If `WebUI` contains `[PORT:<container_port>]`, the planner finds the matching host port.

## Approval workflow

Apply operations require at minimum:

- saved plan;
- `plan_hash`;
- `confirm_plan_hash`;
- backup directory;
- audit directory.

When `require_approval_token` is enabled, a short-lived token created locally on Unraid is also required:

```bash
unraid-ai-manager approve-plan \
  --plan /mnt/user/appdata/unraid-ai-manager/plans/PLAN.json \
  --approvals-dir /mnt/user/appdata/unraid-ai-manager/approvals \
  --purpose amud \
  --ttl 15m
```

The token is stored only as a SHA-256 hash and is marked as used after a successful apply.

## Community Applications plan

The target design is to install applications from Community Applications in a way that preserves Unraid update and metadata behavior.

Required flow:

1. Search the Community Applications catalog.
2. Load the official template.
3. Fill user settings.
4. Show diff, ports, volumes, env, labels and risks.
5. Require explicit approval.
6. Create DockerMan-compatible XML.
7. Create the container through a safe lifecycle mechanism.
8. Verify that Unraid sees the application through the normal Docker/Apps workflow.

Raw Docker create is only a fallback for custom containers outside Community Applications.

## Recreate and lifecycle plan

Recreate will later be a separate write operation with extra approval.

Minimum requirements:

- inspect before the action;
- hashed lifecycle plan;
- user confirmation;
- no changes outside the approved plan;
- inspect after the action;
- audit log;
- clear error reporting if the container does not start after recreate.

## Audit and rollback

Every write must record:

- plan;
- diff;
- original SHA-256;
- new SHA-256;
- backup path;
- timestamp;
- approval identity;
- result.

Backups must not be stored inside `templates-user`, because DockerMan/Community Applications may accidentally treat them as templates.

Recommended paths:

```text
/mnt/user/appdata/unraid-ai-manager/backups
/mnt/user/appdata/unraid-ai-manager/audit
/mnt/user/appdata/unraid-ai-manager/plans
/mnt/user/appdata/unraid-ai-manager/approvals
```
