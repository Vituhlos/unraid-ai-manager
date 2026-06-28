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

func TestApplyTZPlanBacksUpAndWrites(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	backupDir := filepath.Join(dir, "backups")
	auditDir := filepath.Join(dir, "audit")
	if err := os.MkdirAll(templateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templateDir, "my-demo.xml")
	xml := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	plan := planner.BuildTZPlan([]dockerxml.Template{template}, planner.TZOptions{Timezone: "Europe/Prague"})
	report, err := ApplyTZPlan(plan, TZApplyOptions{
		ConfirmPlanHash: plan.PlanHash,
		BackupDir:       backupDir,
		AuditDir:        auditDir,
		Now:             time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || !report.Results[0].Changed {
		t.Fatalf("unexpected report: %#v", report)
	}
	modified, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modified), `Target="TZ"`) {
		t.Fatal("expected TZ variable")
	}
}
