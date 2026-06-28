package planner

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerxml"
)

func TestParseWebUIContainerPort(t *testing.T) {
	port := ParseWebUIContainerPort("http://[IP]:[PORT:9696]/system/status")
	if port == nil || *port != 9696 {
		t.Fatalf("expected 9696, got %#v", port)
	}
	if ParseWebUIContainerPort("") != nil {
		t.Fatal("expected nil port")
	}
}

func TestAMUDPlanLocalURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-demo.xml")
	xml := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <TemplateURL>https://example.test/demo.xml</TemplateURL>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
</Container>`
	if err := os.WriteFile(path, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plan := BuildAMUDPlan([]dockerxml.Template{template}, AMUDOptions{LocalHost: "192.0.2.10"})
	if got := plan.Entries[0].ProposedLabels["amud.url"]; got != "http://192.0.2.10:18080" {
		t.Fatalf("unexpected amud.url: %s", got)
	}
	if got := plan.Entries[0].ProposedLabels["amud.icon"]; got != "demo" {
		t.Fatalf("unexpected amud.icon: %s", got)
	}
}
