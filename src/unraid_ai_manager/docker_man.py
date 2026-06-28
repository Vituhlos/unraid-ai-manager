from __future__ import annotations

from pathlib import Path
from xml.etree import ElementTree

from .models import ConfigEntry, DockerManTemplate


def load_templates(path: Path) -> list[DockerManTemplate]:
    """Load DockerMan templates from a file or non-recursive directory.

    The directory scan is intentionally non-recursive. DockerMan/CA can behave
    surprisingly when XML backups are placed under templates-user, so this tool
    should not silently walk subdirectories.
    """

    if path.is_file():
        return [parse_template(path)]
    if not path.is_dir():
        raise FileNotFoundError(f"Template path does not exist: {path}")

    templates: list[DockerManTemplate] = []
    for xml_path in sorted(path.glob("*.xml")):
        templates.append(parse_template(xml_path))
    return templates


def parse_template(path: Path) -> DockerManTemplate:
    tree = ElementTree.parse(path)
    root = tree.getroot()
    if root.tag != "Container":
        raise ValueError(f"{path} is not a DockerMan Container template")

    configs = tuple(_parse_config(config) for config in root.findall("Config"))

    return DockerManTemplate(
        source_path=path.resolve(),
        version=root.attrib.get("version", ""),
        name=_child_text(root, "Name"),
        repository=_child_text(root, "Repository"),
        registry=_child_text(root, "Registry"),
        network=_child_text(root, "Network"),
        my_ip=_child_text(root, "MyIP"),
        shell=_child_text(root, "Shell"),
        privileged=_child_text(root, "Privileged").lower() == "true",
        support=_child_text(root, "Support"),
        project=_child_text(root, "Project"),
        readme=_child_text(root, "ReadMe"),
        overview=_child_text(root, "Overview"),
        category=_child_text(root, "Category"),
        web_ui=_child_text(root, "WebUI"),
        template_url=_child_text(root, "TemplateURL"),
        icon=_child_text(root, "Icon"),
        extra_params=_child_text(root, "ExtraParams"),
        post_args=_child_text(root, "PostArgs"),
        cpu_set=_child_text(root, "CPUset"),
        date_installed=_child_text(root, "DateInstalled"),
        configs=configs,
    )


def _parse_config(element: ElementTree.Element) -> ConfigEntry:
    attrs = {key: value for key, value in element.attrib.items()}
    return ConfigEntry(
        name=attrs.get("Name", ""),
        target=attrs.get("Target", ""),
        default=attrs.get("Default", ""),
        mode=attrs.get("Mode", ""),
        description=attrs.get("Description", ""),
        type=attrs.get("Type", ""),
        display=attrs.get("Display", ""),
        required=attrs.get("Required", ""),
        mask=attrs.get("Mask", ""),
        value=(element.text or "").strip(),
        attrs=attrs,
    )


def _child_text(root: ElementTree.Element, tag: str) -> str:
    child = root.find(tag)
    if child is None or child.text is None:
        return ""
    return child.text.strip()

