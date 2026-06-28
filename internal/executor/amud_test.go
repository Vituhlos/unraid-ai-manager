package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/planner"
)

func TestApplyAMUDPlanRequiresConfirmHash(t *testing.T) {
	_, err := ApplyAMUDPlan(planner.AMUDPlan{
		Kind:     "amud-labels",
		PlanHash: "abc",
	}, AMUDApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "--confirm-plan-hash") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestApplyAMUDPlanBacksUpAndWrites(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	backupDir := filepath.Join(dir, "backups")
	auditDir := filepath.Join(dir, "audit")
	if err := os.MkdirAll(templateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templateDir, "my-demo.xml")
	original := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <TemplateURL>https://example.test/demo.xml</TemplateURL>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	template, err := dockerxml.ParseTemplateFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	plan := planner.BuildAMUDPlan([]dockerxml.Template{template}, planner.AMUDOptions{LocalHost: "192.0.2.10"})
	report, err := ApplyAMUDPlan(plan, AMUDApplyOptions{
		ConfirmPlanHash: plan.PlanHash,
		BackupDir:       backupDir,
		AuditDir:        auditDir,
		Now:             time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || !report.Results[0].Changed {
		t.Fatalf("expected one changed result, got %#v", report.Results)
	}

	modified, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modified), `Target="amud.url"`) {
		t.Fatal("expected amud.url in modified template")
	}
	backups, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected one backup, got %d", len(backups))
	}
	audits, err := os.ReadDir(auditDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one audit file, got %d", len(audits))
	}
}
