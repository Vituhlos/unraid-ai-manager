package planner

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerxml"
)

func TestBuildTZPlanAddAndUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-demo.xml")
	xml := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Config Name="TZ" Target="TZ" Default="UTC" Mode="" Description="" Type="Variable" Display="advanced" Required="false" Mask="false">UTC</Config>
</Container>
`
	if err := os.WriteFile(path, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plan := BuildTZPlan([]dockerxml.Template{template}, TZOptions{Timezone: "Europe/Prague"})
	if len(plan.Entries) != 1 {
		t.Fatalf("expected one entry, got %#v", plan)
	}
	if plan.Entries[0].Action != "update" {
		t.Fatalf("expected update, got %s", plan.Entries[0].Action)
	}
}
