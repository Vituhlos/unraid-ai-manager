from __future__ import annotations

import hashlib
import json
import re
from dataclasses import dataclass
from typing import Any, Literal

from .models import ConfigEntry, DockerManTemplate
from .risk import analyze_template_risks

UrlMode = Literal["local", "cloudflare", "hybrid"]


@dataclass(frozen=True)
class AmudOptions:
    local_host: str
    url_mode: UrlMode = "local"
    cloudflare_domain: str = ""
    cloudflare_routes: dict[str, str] | None = None


def build_amud_plan(templates: list[DockerManTemplate], options: AmudOptions) -> dict[str, Any]:
    entries: list[dict[str, Any]] = []

    for template in templates:
        web = detect_web_candidate(template)
        if web["confidence"] == "none":
            continue

        url_result = resolve_amud_url(template, options)
        proposed_labels = {
            "amud.enable": "true",
            "amud.url": url_result["url"] or "",
            "amud.name": template.name,
            "amud.icon": infer_icon_name(template),
        }

        if not url_result["url"]:
            proposed_labels.pop("amud.url")

        current_labels = template.label_map()
        label_changes = []
        for key, proposed_value in proposed_labels.items():
            current_value = current_labels.get(key)
            if current_value is None:
                action = "add"
            elif current_value == proposed_value:
                action = "unchanged"
            else:
                action = "update"
            label_changes.append(
                {
                    "action": action,
                    "key": key,
                    "current": current_value,
                    "proposed": proposed_value,
                }
            )

        warnings = list(url_result["warnings"])
        risk_findings = analyze_template_risks(template)
        warnings.extend(
            f"{finding.severity.upper()}: {finding.message}" for finding in risk_findings if finding.severity in {"high", "review"}
        )

        entries.append(
            {
                "container": template.name,
                "source_path": str(template.source_path),
                "repository": template.repository,
                "template_url": template.template_url,
                "web_detection": web,
                "url": url_result,
                "current_labels": current_labels,
                "proposed_labels": proposed_labels,
                "label_changes": label_changes,
                "warnings": warnings,
            }
        )

    plan: dict[str, Any] = {
        "kind": "amud-labels",
        "write_enabled": False,
        "url_mode": options.url_mode,
        "local_host": options.local_host,
        "cloudflare_domain": options.cloudflare_domain,
        "entries": entries,
    }
    plan["plan_hash"] = _hash_plan(plan)
    return plan


def detect_web_candidate(template: DockerManTemplate) -> dict[str, Any]:
    if template.web_ui:
        return {
            "confidence": "high",
            "reason": "Template has WebUI.",
            "web_ui": template.web_ui,
            "container_port": parse_webui_container_port(template.web_ui),
        }

    tcp_ports = [port for port in template.ports if port.mode.lower() == "tcp"]
    if tcp_ports:
        return {
            "confidence": "medium",
            "reason": "Template has published TCP port but no WebUI.",
            "web_ui": "",
            "container_port": _safe_int(tcp_ports[0].target),
        }

    return {
        "confidence": "none",
        "reason": "No WebUI and no TCP ports.",
        "web_ui": "",
        "container_port": None,
    }


def resolve_amud_url(template: DockerManTemplate, options: AmudOptions) -> dict[str, Any]:
    warnings: list[str] = []
    routes = options.cloudflare_routes or {}

    if options.url_mode in {"cloudflare", "hybrid"}:
        route = _lookup_route(routes, template.name)
        if route:
            return {
                "mode": "cloudflare",
                "source": "explicit-route",
                "url": _cloudflare_url(route, options.cloudflare_domain),
                "warnings": warnings,
            }
        if options.url_mode == "cloudflare":
            warnings.append(f"No Cloudflare route known for {template.name}; amud.url will not be proposed.")
            return {"mode": "cloudflare", "source": "missing-route", "url": "", "warnings": warnings}

    local = resolve_local_url(template, options.local_host)
    warnings.extend(local["warnings"])
    return {
        "mode": "local",
        "source": local["source"],
        "url": local["url"],
        "host_port": local["host_port"],
        "container_port": local["container_port"],
        "warnings": warnings,
    }


def resolve_local_url(template: DockerManTemplate, local_host: str) -> dict[str, Any]:
    warnings: list[str] = []
    container_port = parse_webui_container_port(template.web_ui)
    selected_port: ConfigEntry | None = None

    if container_port is not None:
        selected_port = _find_port_by_container_port(template, container_port)

    if selected_port is None and template.ports:
        selected_port = next((port for port in template.ports if port.mode.lower() == "tcp"), template.ports[0])
        warnings.append("WebUI port did not match a Port config; using first TCP/available port.")

    if selected_port is None:
        if template.network.lower() == "host" and container_port:
            warnings.append("Host network template; using WebUI container port as host port.")
            return {
                "source": "host-network-webui",
                "url": f"http://{local_host}:{container_port}",
                "host_port": str(container_port),
                "container_port": str(container_port),
                "warnings": warnings,
            }
        warnings.append("No usable port found for local AMUD URL.")
        return {"source": "missing-port", "url": "", "host_port": "", "container_port": container_port, "warnings": warnings}

    host_port = selected_port.value
    if not host_port:
        warnings.append(f"Port config {selected_port.name} has empty host port.")
        return {
            "source": "empty-host-port",
            "url": "",
            "host_port": "",
            "container_port": selected_port.target,
            "warnings": warnings,
        }

    return {
        "source": "webui-port" if container_port is not None else "first-port",
        "url": f"http://{local_host}:{host_port}",
        "host_port": host_port,
        "container_port": selected_port.target,
        "warnings": warnings,
    }


def parse_webui_container_port(web_ui: str) -> int | None:
    match = re.search(r"\[PORT:(\d+)\]", web_ui)
    if not match:
        return None
    return int(match.group(1))


def infer_icon_name(template: DockerManTemplate) -> str:
    normalized = re.sub(r"[^a-z0-9]+", "-", template.name.lower()).strip("-")
    return normalized or template.name.lower()


def _find_port_by_container_port(template: DockerManTemplate, container_port: int) -> ConfigEntry | None:
    target = str(container_port)
    tcp_match = next((port for port in template.ports if port.target == target and port.mode.lower() == "tcp"), None)
    if tcp_match is not None:
        return tcp_match
    return next((port for port in template.ports if port.target == target), None)


def _lookup_route(routes: dict[str, str], name: str) -> str:
    for key in (name, name.lower(), infer_route_key(name)):
        if key in routes:
            return routes[key]
    return ""


def infer_route_key(name: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")


def _cloudflare_url(route: str, domain: str) -> str:
    if route.startswith(("http://", "https://")):
        return route
    if not domain:
        return f"https://{route}"
    return f"https://{route}.{domain}"


def _safe_int(value: str) -> int | None:
    try:
        return int(value)
    except ValueError:
        return None


def _hash_plan(plan: dict[str, Any]) -> str:
    payload = {key: value for key, value in plan.items() if key != "plan_hash"}
    encoded = json.dumps(payload, sort_keys=True, ensure_ascii=False).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()[:16]

