package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/discovery"
	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/planner"
)

func TestBuildDashboardIntegrationPlanUsesSecretRefs(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "radarr")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.xml"), []byte(`<Config><ApiKey>abcdef1234567890</ApiKey></Config>`), 0o600); err != nil {
		t.Fatal(err)
	}
	template := dockerxml.Template{
		Name:       "radarr",
		Repository: "lscr.io/linuxserver/radarr:latest",
		WebUI:      "http://[IP]:[PORT:7878]/",
		Configs: []dockerxml.ConfigEntry{
			{Type: "Port", Target: "7878", Mode: "tcp", Value: "7878"},
			{Type: "Path", Target: "/config", Value: configDir},
		},
	}
	dashboardPlan, err := planner.BuildDashboardPlan([]dockerxml.Template{template}, planner.DashboardOptions{
		Provider:  "amud",
		LocalHost: "192.0.2.10",
		URLMode:   "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	discoveryReport := discovery.DiscoverIntegrations([]dockerxml.Template{template}, discovery.Options{})
	plan := BuildDashboardIntegrationPlan(dashboardPlan, discoveryReport)
	if len(plan.Entries) != 1 {
		t.Fatalf("expected one entry, got %#v", plan.Entries)
	}
	entry := plan.Entries[0]
	if entry.Status != "ready" {
		t.Fatalf("expected ready status, got %#v", entry)
	}
	if len(entry.RequiredSecrets) != 1 || entry.RequiredSecrets[0].Ref == "" {
		t.Fatalf("expected secret ref, got %#v", entry.RequiredSecrets)
	}
	if entry.RequiredSecrets[0].Preview == "abcdef1234567890" {
		t.Fatal("integration plan leaked full API key")
	}
}
