from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from .docker_man import load_templates
from .planner import AmudOptions, build_amud_plan
from .risk import analyze_template_risks


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="unraid-ai-manager")
    subcommands = parser.add_subparsers(dest="command", required=True)

    inventory = subcommands.add_parser("inventory", help="Read DockerMan XML templates and print inventory.")
    inventory.add_argument("--templates", required=True, help="Path to templates-user directory or one XML file.")
    inventory.add_argument("--json", action="store_true", help="Print machine-readable JSON.")

    plan_amud = subcommands.add_parser("plan-amud", help="Create a read-only AMUD label plan.")
    plan_amud.add_argument("--templates", required=True, help="Path to templates-user directory or one XML file.")
    plan_amud.add_argument("--local-host", required=True, help="Local Unraid host/IP for local AMUD URLs.")
    plan_amud.add_argument("--url-mode", choices=["local", "cloudflare", "hybrid"], default="local")
    plan_amud.add_argument("--cloudflare-domain", default="", help="Cloudflare base domain, e.g. pbas.cz.")
    plan_amud.add_argument(
        "--route",
        action="append",
        default=[],
        metavar="NAME=SUBDOMAIN_OR_URL",
        help="Cloudflare mapping. Example: prowlarr=prowlarr or Seerr=https://seerr.pbas.cz",
    )
    plan_amud.add_argument("--json", action="store_true", help="Print machine-readable JSON.")

    args = parser.parse_args(argv)

    if args.command == "inventory":
        templates = load_templates(Path(args.templates))
        payload = build_inventory_payload(templates)
        if args.json:
            print(json.dumps(payload, indent=2, ensure_ascii=False))
        else:
            print_inventory(payload)
        return 0

    if args.command == "plan-amud":
        templates = load_templates(Path(args.templates))
        routes = parse_routes(args.route)
        plan = build_amud_plan(
            templates,
            AmudOptions(
                local_host=args.local_host,
                url_mode=args.url_mode,
                cloudflare_domain=args.cloudflare_domain,
                cloudflare_routes=routes,
            ),
        )
        if args.json:
            print(json.dumps(plan, indent=2, ensure_ascii=False))
        else:
            print_amud_plan(plan)
        return 0

    parser.error(f"Unknown command: {args.command}")
    return 2


def build_inventory_payload(templates: list[Any]) -> dict[str, Any]:
    containers = []
    for template in templates:
        risks = analyze_template_risks(template)
        containers.append(
            {
                **template.to_summary_dict(),
                "risk_findings": [risk.to_dict() for risk in risks],
            }
        )
    return {
        "write_enabled": False,
        "template_count": len(containers),
        "containers": containers,
    }


def print_inventory(payload: dict[str, Any]) -> None:
    print("Unraid AI Manager inventory (read-only)")
    print(f"Templates: {payload['template_count']}")
    print()

    if not payload["containers"]:
        print("No templates found.")
        return

    for container in payload["containers"]:
        ports = ", ".join(f"{port['value']}->{port['target']}/{port['mode']}" for port in container["ports"]) or "-"
        risks = _risk_summary(container["risk_findings"])
        print(f"- {container['name']}")
        print(f"  Repository:  {container['repository']}")
        print(f"  Network:     {container['network'] or '-'}")
        print(f"  WebUI:       {container['web_ui'] or '-'}")
        print(f"  Ports:       {ports}")
        print(f"  TemplateURL: {container['template_url'] or '-'}")
        print(f"  Risks:       {risks}")
        print()


def print_amud_plan(plan: dict[str, Any]) -> None:
    print("AMUD label plan (read-only)")
    print(f"Plan hash: {plan['plan_hash']}")
    print(f"URL mode:  {plan['url_mode']}")
    print(f"Local:     {plan['local_host']}")
    if plan["cloudflare_domain"]:
        print(f"Cloudflare domain: {plan['cloudflare_domain']}")
    print()

    if not plan["entries"]:
        print("No web container candidates found.")
        return

    for entry in plan["entries"]:
        print(f"- {entry['container']}")
        print(f"  Detection: {entry['web_detection']['confidence']} ({entry['web_detection']['reason']})")
        print(f"  URL:       {entry['url']['url'] or '-'}")
        print("  Labels:")
        for change in entry["label_changes"]:
            prefix = {"add": "+", "update": "~", "unchanged": "="}.get(change["action"], "?")
            current = "" if change["current"] is None else f" (current: {change['current']})"
            print(f"    {prefix} {change['key']}={change['proposed']}{current}")
        if entry["warnings"]:
            print("  Warnings:")
            for warning in entry["warnings"]:
                print(f"    - {warning}")
        print()

    print("No files were changed.")


def parse_routes(values: list[str]) -> dict[str, str]:
    routes: dict[str, str] = {}
    for value in values:
        if "=" not in value:
            raise ValueError(f"Invalid --route value, expected NAME=SUBDOMAIN_OR_URL: {value}")
        key, route = value.split("=", 1)
        key = key.strip()
        route = route.strip()
        if not key or not route:
            raise ValueError(f"Invalid --route value, expected non-empty key and route: {value}")
        routes[key] = route
        routes[key.lower()] = route
    return routes


def _risk_summary(findings: list[dict[str, str]]) -> str:
    if not findings:
        return "none"
    counts: dict[str, int] = {}
    for finding in findings:
        counts[finding["severity"]] = counts.get(finding["severity"], 0) + 1
    return ", ".join(f"{severity}:{count}" for severity, count in sorted(counts.items()))


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

