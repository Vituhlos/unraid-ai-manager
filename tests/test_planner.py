from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from unraid_ai_manager.docker_man import parse_template
from unraid_ai_manager.planner import AmudOptions, build_amud_plan, parse_webui_container_port


class PlannerTests(unittest.TestCase):
    def test_parse_webui_container_port(self) -> None:
        self.assertEqual(parse_webui_container_port("http://[IP]:[PORT:9696]/system/status"), 9696)
        self.assertIsNone(parse_webui_container_port(""))

    def test_amud_plan_local_url(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            xml = Path(temp_dir) / "my-demo.xml"
            xml.write_text(
                """<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <TemplateURL>https://example.test/demo.xml</TemplateURL>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
</Container>
""",
                encoding="utf-8",
            )

            template = parse_template(xml)
            plan = build_amud_plan([template], AmudOptions(local_host="192.168.1.114"))

            self.assertEqual(plan["entries"][0]["proposed_labels"]["amud.url"], "http://192.168.1.114:18080")
            self.assertEqual(plan["entries"][0]["proposed_labels"]["amud.icon"], "demo")


if __name__ == "__main__":
    unittest.main()
