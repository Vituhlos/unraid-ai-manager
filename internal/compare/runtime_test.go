package compare

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
)

func TestRuntimeVsTemplatesMatchesPortsAndLabels(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "my-prowlarr.xml")
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>prowlarr</Name>
  <Repository>lscr.io/linuxserver/prowlarr</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <Config Name="WebUI" Target="9696" Default="9696" Mode="tcp" Type="Port">9696</Config>
  <Config Name="AMUD Enable" Target="amud.enable" Default="true" Mode="" Description="" Type="Label" Display="advanced" Required="false" Mask="false">true</Config>
</Container>
`
	inspectPath := filepath.Join(dir, "inspect.json")
	inspectJSON := `[
  {
    "Id": "abc123",
    "Name": "/prowlarr",
    "Config": {
      "Image": "lscr.io/linuxserver/prowlarr",
      "Labels": {"amud.enable": "true"},
      "Env": []
    },
    "State": {"Status": "running"},
    "HostConfig": {"NetworkMode": "bridge"},
    "NetworkSettings": {"Ports": {"9696/tcp": [{"HostIp": "0.0.0.0", "HostPort": "9696"}]}}
  }
]`
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inspectPath, []byte(inspectJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	containers, err := dockerinspect.LoadFile(inspectPath)
	if err != nil {
		t.Fatal(err)
	}
	report := RuntimeVsTemplates([]dockerxml.Template{template}, containers)
	if len(report.Matches) != 1 {
		t.Fatalf("expected one match, got %#v", report)
	}
	if !report.Matches[0].PortComparisons[0].Match {
		t.Fatalf("expected port match: %#v", report.Matches[0].PortComparisons[0])
	}
	if !report.Matches[0].LabelComparisons[0].Match {
		t.Fatalf("expected label match: %#v", report.Matches[0].LabelComparisons[0])
	}
}
