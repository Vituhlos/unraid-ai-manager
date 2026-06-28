from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class ConfigEntry:
    name: str
    target: str
    default: str
    mode: str
    description: str
    type: str
    display: str
    required: str
    mask: str
    value: str
    attrs: dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "target": self.target,
            "default": self.default,
            "mode": self.mode,
            "description": self.description,
            "type": self.type,
            "display": self.display,
            "required": self.required,
            "mask": self.mask,
            "value": self.value,
        }


@dataclass(frozen=True)
class DockerManTemplate:
    source_path: Path
    version: str
    name: str
    repository: str
    registry: str
    network: str
    my_ip: str
    shell: str
    privileged: bool
    support: str
    project: str
    readme: str
    overview: str
    category: str
    web_ui: str
    template_url: str
    icon: str
    extra_params: str
    post_args: str
    cpu_set: str
    date_installed: str
    configs: tuple[ConfigEntry, ...]

    @property
    def ports(self) -> tuple[ConfigEntry, ...]:
        return tuple(config for config in self.configs if config.type.lower() == "port")

    @property
    def paths(self) -> tuple[ConfigEntry, ...]:
        return tuple(config for config in self.configs if config.type.lower() == "path")

    @property
    def variables(self) -> tuple[ConfigEntry, ...]:
        return tuple(config for config in self.configs if config.type.lower() == "variable")

    @property
    def labels(self) -> tuple[ConfigEntry, ...]:
        return tuple(config for config in self.configs if config.type.lower() == "label")

    def label_map(self) -> dict[str, str]:
        return {config.target: config.value for config in self.labels if config.target}

    def variable_map(self) -> dict[str, str]:
        return {config.target: config.value for config in self.variables if config.target}

    def to_summary_dict(self) -> dict[str, Any]:
        return {
            "source_path": str(self.source_path),
            "version": self.version,
            "name": self.name,
            "repository": self.repository,
            "registry": self.registry,
            "network": self.network,
            "privileged": self.privileged,
            "web_ui": self.web_ui,
            "template_url": self.template_url,
            "icon": self.icon,
            "extra_params": self.extra_params,
            "ports": [port.to_dict() for port in self.ports],
            "paths": [path.to_dict() for path in self.paths],
            "variables": [variable.to_dict() for variable in self.variables],
            "labels": [label.to_dict() for label in self.labels],
        }


@dataclass(frozen=True)
class RiskFinding:
    severity: str
    code: str
    message: str
    field: str = ""

    def to_dict(self) -> dict[str, str]:
        return {
            "severity": self.severity,
            "code": self.code,
            "message": self.message,
            "field": self.field,
        }

