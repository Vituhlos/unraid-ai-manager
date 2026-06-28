from __future__ import annotations

import shlex

from .models import DockerManTemplate, RiskFinding


SAFE_EXTRA_PARAMS = {
    "--init",
}

SAFE_EXTRA_PARAM_PREFIXES = (
    "--user=",
    "--restart=",
)

SAFE_EXTRA_PARAMS_WITH_VALUE = {
    "--user",
}

DANGEROUS_EXTRA_PARAM_PREFIXES = (
    "--privileged",
    "--cap-add",
    "--device",
    "--mount",
    "--volume",
    "-v",
    "--pid=host",
    "--ipc=host",
    "--uts=host",
    "--network=host",
)


def analyze_template_risks(template: DockerManTemplate) -> list[RiskFinding]:
    findings: list[RiskFinding] = []

    if template.privileged:
        findings.append(
            RiskFinding(
                severity="high",
                code="privileged_container",
                message="Template enables Privileged=true.",
                field="Privileged",
            )
        )

    if template.network.lower() == "host":
        findings.append(
            RiskFinding(
                severity="review",
                code="host_network",
                message="Container uses host networking; port inference and exposure need review.",
                field="Network",
            )
        )

    findings.extend(_analyze_extra_params(template.extra_params))
    findings.extend(_analyze_paths(template))

    if not template.template_url:
        findings.append(
            RiskFinding(
                severity="info",
                code="missing_template_url",
                message="TemplateURL is empty; this may be a custom/non-CA template.",
                field="TemplateURL",
            )
        )

    return findings


def _analyze_extra_params(extra_params: str) -> list[RiskFinding]:
    if not extra_params:
        return []

    findings: list[RiskFinding] = []
    try:
        tokens = shlex.split(extra_params, posix=False)
    except ValueError:
        findings.append(
            RiskFinding(
                severity="review",
                code="extra_params_parse_failed",
                message=f"ExtraParams could not be parsed safely: {extra_params}",
                field="ExtraParams",
            )
        )
        return findings

    index = 0
    while index < len(tokens):
        token = tokens[index]
        if token in SAFE_EXTRA_PARAMS or token.startswith(SAFE_EXTRA_PARAM_PREFIXES):
            index += 1
            continue
        if token in SAFE_EXTRA_PARAMS_WITH_VALUE:
            index += 2
            continue
        if token.startswith(DANGEROUS_EXTRA_PARAM_PREFIXES):
            findings.append(
                RiskFinding(
                    severity="high",
                    code="dangerous_extra_param",
                    message=f"ExtraParams contains potentially dangerous option: {token}",
                    field="ExtraParams",
                )
            )
        else:
            findings.append(
                RiskFinding(
                    severity="review",
                    code="unclassified_extra_param",
                    message=f"ExtraParams contains option that should be reviewed: {token}",
                    field="ExtraParams",
                )
            )
        index += 1
    return findings


def _analyze_paths(template: DockerManTemplate) -> list[RiskFinding]:
    findings: list[RiskFinding] = []
    for path_config in template.paths:
        host_path = _normalize_host_path(path_config.value)
        if not host_path:
            continue

        if host_path in {"/", "/boot", "/etc", "/usr", "/var", "/mnt", "/mnt/user", "/mnt/cache"}:
            findings.append(
                RiskFinding(
                    severity="high",
                    code="broad_host_mount",
                    message=f"Path maps a broad host location: {host_path}",
                    field=path_config.name,
                )
            )
            continue

        if host_path == "/var/run/docker.sock":
            findings.append(
                RiskFinding(
                    severity="high",
                    code="docker_socket_mount",
                    message="Path maps /var/run/docker.sock.",
                    field=path_config.name,
                )
            )
            continue

        if host_path.startswith(("/boot/", "/etc/", "/usr/", "/var/run/")):
            findings.append(
                RiskFinding(
                    severity="high",
                    code="sensitive_host_mount",
                    message=f"Path maps sensitive host location: {host_path}",
                    field=path_config.name,
                )
            )
            continue

        if host_path.startswith("/mnt/user/") and not host_path.startswith("/mnt/user/appdata/"):
            severity = "review" if path_config.mode.lower() in {"rw", "rw,slave", "rw,shared"} else "info"
            findings.append(
                RiskFinding(
                    severity=severity,
                    code="user_share_mount",
                    message=f"Path maps a user share location: {host_path}",
                    field=path_config.name,
                )
            )

    return findings


def _normalize_host_path(path: str) -> str:
    normalized = path.replace("\\", "/").strip()
    while "//" in normalized:
        normalized = normalized.replace("//", "/")
    if normalized != "/":
        normalized = normalized.rstrip("/")
    return normalized
