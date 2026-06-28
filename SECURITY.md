# Security Policy

Unraid AI Manager is designed as a privileged local automation tool. Treat it with the same care as any tool that can modify Unraid Docker templates.

## Supported versions

Only the latest released version is supported while the project is in the `0.y.z` preview phase.

## Security model

The AI client and MCP server are not trusted security boundaries.

The Unraid-side helper daemon is the enforcement point. It must:

- expose only whitelisted actions;
- reject raw shell commands;
- require exact plan hashes for apply operations;
- create backups before XML writes;
- write audit logs;
- optionally require short-lived local approval tokens;
- avoid exposing unrestricted Docker socket access to MCP clients.

## Default network posture

The helper daemon should bind to:

```text
127.0.0.1:37231
```

Recommended remote access is through an SSH tunnel:

```bash
ssh -L 37231:127.0.0.1:37231 root@<unraid-ip>
```

Do not expose the helper directly to the public internet.

## High-risk operations

These operations must remain blocked or require explicit extra approval:

- raw shell execution;
- privileged containers;
- host filesystem mounts such as `/`, `/boot`, `/etc`, `/usr`;
- unrestricted `/var/run/docker.sock` exposure;
- whole `/mnt` or `/mnt/user` mounts;
- host networking;
- arbitrary device and capability additions;
- arbitrary container start/stop/remove operations;
- container recreate outside an approved DockerMan recreate plan;
- Community Applications installation;
- plugin installation;
- disk, array, VM or share destructive actions.

## Reporting a vulnerability

For now, report vulnerabilities privately to the repository owner through GitHub.

Please include:

- affected version;
- reproduction steps;
- expected impact;
- whether the issue requires local access, LAN access or remote access;
- suggested mitigation if known.

Do not publish exploit details before a fixed version is available.
