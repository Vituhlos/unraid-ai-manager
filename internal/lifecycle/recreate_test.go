package lifecycle

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
)

func TestBuildRecreatePlanIncludesLabelDiff(t *testing.T) {
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
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	runtime := []dockerinspect.Container{{
		Name:        "prowlarr",
		Image:       "lscr.io/linuxserver/prowlarr",
		State:       "running",
		NetworkMode: "bridge",
		Labels:      map[string]string{},
		Ports: []dockerinspect.Port{{
			ContainerPort: "9696",
			Protocol:      "tcp",
			HostPort:      "9696",
		}},
	}}

	plan := BuildRecreatePlan([]dockerxml.Template{template}, runtime, Options{})
	if len(plan.Entries) != 1 {
		t.Fatalf("expected one recreate entry, got %#v", plan)
	}
	if len(plan.Entries[0].Reasons) == 0 {
		t.Fatal("expected recreate reasons")
	}
	if plan.PlanHash == "" {
		t.Fatal("expected plan hash")
	}
}
