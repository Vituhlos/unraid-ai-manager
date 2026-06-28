package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/planner"
	"unraid-ai-manager/internal/workflow"
)

type fakeSyncRuntime struct {
	inspectCalls int
	started      bool
}

func (f *fakeSyncRuntime) InspectContainer(ctx context.Context, id string) (dockerinspect.Container, error) {
	f.inspectCalls++
	labels := map[string]string{}
	if f.inspectCalls >= 2 {
		labels = map[string]string{
			"amud.enable": "true",
			"amud.icon":   "radarr",
			"amud.name":   "radarr",
			"amud.url":    "http://192.0.2.10:7878",
		}
	}
	return dockerinspect.Container{
		Name:   id,
		State:  "running",
		Labels: labels,
	}, nil
}

func (f *fakeSyncRuntime) StartContainer(ctx context.Context, id string) error {
	f.started = true
	return nil
}

type fakeSyncRunner struct {
	rebuilt []string
}

func (f *fakeSyncRunner) Rebuild(ctx context.Context, scriptPath string, container string) (string, error) {
	f.rebuilt = append(f.rebuilt, container)
	return "rebuilt " + container, nil
}

func TestApplyDashboardSyncPlanAppliesXMLRecreatesAndVerifies(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templatesDir, "my-radarr.xml")
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>radarr</Name>
  <Repository>lscr.io/linuxserver/radarr:latest</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:7878]/</WebUI>
  <Config Name="WebUI" Target="7878" Default="7878" Mode="tcp" Type="Port">7878</Config>
  <Config Name="Config" Target="/config" Default="/mnt/user/appdata/radarr" Mode="rw" Type="Path">/mnt/user/appdata/radarr</Config>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}
	templates, err := dockerxml.LoadTemplates(templatesDir)
	if err != nil {
		t.Fatal(err)
	}
	runtime := []dockerinspect.Container{
		{
			Name:  "radarr",
			Image: "lscr.io/linuxserver/radarr:latest",
			State: "running",
			Ports: []dockerinspect.Port{
				{ContainerPort: "7878", Protocol: "tcp", HostPort: "7878"},
			},
			Labels: map[string]string{},
		},
	}
	dashboardPlan, err := planner.BuildDashboardPlan(templates, planner.DashboardOptions{
		Provider:      "amud",
		LocalHost:     "192.0.2.10",
		URLMode:       "local",
		RuntimeFilter: "running",
	})
	if err != nil {
		t.Fatal(err)
	}
	syncPlan, err := workflow.BuildDashboardSyncPlan(templates, runtime, dashboardPlan, workflow.DashboardSyncOptions{
		RecreateMode: workflow.RecreateModeChanged,
	})
	if err != nil {
		t.Fatal(err)
	}
	if syncPlan.PlanHash == "" {
		t.Fatal("missing sync plan hash")
	}
	if len(syncPlan.RecreatePlan.Entries) != 1 {
		t.Fatalf("expected one recreate entry, got %#v", syncPlan.RecreatePlan.Entries)
	}

	fakeRuntime := &fakeSyncRuntime{}
	fakeRunner := &fakeSyncRunner{}
	report, err := ApplyDashboardSyncPlan(context.Background(), syncPlan, DashboardSyncApplyOptions{
		ConfirmPlanHash: syncPlan.PlanHash,
		BackupDir:       filepath.Join(dir, "backups"),
		AuditDir:        filepath.Join(dir, "audit"),
		Runtime:         fakeRuntime,
		Runner:          fakeRunner,
		RebuildScript:   filepath.Join(dir, "rebuild_container"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected OK report, got %#v", report)
	}
	if len(fakeRunner.rebuilt) != 1 || fakeRunner.rebuilt[0] != "radarr" {
		t.Fatalf("expected radarr rebuild, got %#v", fakeRunner.rebuilt)
	}
	modified, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modified), `Target="amud.url"`) {
		t.Fatal("expected AMUD URL label to be written to XML")
	}
	if len(report.Verification) != 1 || report.Verification[0].Labels["amud.url"] != "http://192.0.2.10:7878" {
		t.Fatalf("expected runtime verification labels, got %#v", report.Verification)
	}
}
