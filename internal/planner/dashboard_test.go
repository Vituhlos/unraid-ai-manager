package planner

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerxml"
)

func TestDashboardPlanUsesAMUDAdapter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-radarr.xml")
	xml := `<?xml version="1.0"?>
<Container version="2">
  <Name>radarr</Name>
  <Repository>lscr.io/linuxserver/radarr</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:7878]/</WebUI>
  <Config Name="WebUI" Target="7878" Default="7878" Mode="tcp" Type="Port">7878</Config>
</Container>`
	if err := os.WriteFile(path, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(path)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := BuildDashboardPlan([]dockerxml.Template{template}, DashboardOptions{
		Provider:  "amud",
		LocalHost: "192.0.2.10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Kind != "dashboard-config" {
		t.Fatalf("unexpected kind: %s", plan.Kind)
	}
	if plan.Provider != DashboardProviderAMUD || plan.Adapter != DashboardAdapterAMUD {
		t.Fatalf("unexpected provider/adapter: %s/%s", plan.Provider, plan.Adapter)
	}
	if len(plan.Entries) != 1 {
		t.Fatalf("expected one entry, got %#v", plan.Entries)
	}
	entry := plan.Entries[0]
	if entry.Service.IntegrationType != "radarr" {
		t.Fatalf("expected radarr integration, got %#v", entry.Service)
	}
	if entry.ProposedState["amud.url"] != "http://192.0.2.10:7878" {
		t.Fatalf("unexpected proposed URL: %#v", entry.ProposedState)
	}
	if len(entry.TargetChanges) == 0 || entry.TargetChanges[0].TargetType != "docker_label" {
		t.Fatalf("expected docker label target changes, got %#v", entry.TargetChanges)
	}
	if plan.PlanHash == "" {
		t.Fatal("missing dashboard plan hash")
	}
}

func TestAMUDPlanFromDashboardPlan(t *testing.T) {
	dashboardPlan := DashboardPlan{
		Kind:          "dashboard-config",
		Provider:      "amud",
		Adapter:       DashboardAdapterAMUD,
		URLMode:       "local",
		LocalHost:     "192.0.2.10",
		RuntimeFilter: "templates",
		PlanHash:      "abc123",
		Entries: []DashboardEntry{
			{
				Container:  "radarr",
				SourcePath: "/tmp/my-radarr.xml",
				ProposedState: map[string]string{
					"amud.enable": "true",
					"amud.url":    "http://192.0.2.10:7878",
				},
				TargetChanges: []DashboardTargetChange{
					{Action: "add", TargetType: "docker_label", Key: "amud.enable", Proposed: "true"},
				},
			},
		},
	}

	amud, err := AMUDPlanFromDashboardPlan(dashboardPlan)
	if err != nil {
		t.Fatal(err)
	}
	if amud.Kind != "amud-labels" {
		t.Fatalf("unexpected AMUD kind: %s", amud.Kind)
	}
	if amud.PlanHash != dashboardPlan.PlanHash {
		t.Fatalf("expected dashboard hash to be preserved")
	}
	if amud.Entries[0].ProposedLabels["amud.url"] != "http://192.0.2.10:7878" {
		t.Fatalf("missing AMUD proposed labels: %#v", amud.Entries[0].ProposedLabels)
	}
}
