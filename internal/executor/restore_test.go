package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRestoreXMLBackupRequiresConfirmedSHA(t *testing.T) {
	_, err := RestoreXMLBackup(RestoreXMLOptions{})
	if err == nil || !strings.Contains(err.Error(), "--backup") {
		t.Fatalf("expected backup required error, got %v", err)
	}
}

func TestRestoreXMLBackupRestoresAndAudits(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	restoreBackups := filepath.Join(dir, "restore-backups")
	auditDir := filepath.Join(dir, "audit")
	if err := os.MkdirAll(templateDir, 0o700); err != nil {
		t.Fatal(err)
	}

	backupXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
</Container>
`
	modifiedXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Config Name="AMUD URL" Target="amud.url" Default="http://x" Mode="" Description="" Type="Label" Display="advanced" Required="false" Mask="false">http://x</Config>
</Container>
`
	backupPath := filepath.Join(dir, "backup.xml")
	targetPath := filepath.Join(templateDir, "my-demo.xml")
	if err := os.WriteFile(backupPath, []byte(backupXML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte(modifiedXML), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := RestoreXMLBackup(RestoreXMLOptions{
		BackupPath:          backupPath,
		TargetPath:          targetPath,
		ConfirmBackupSHA256: sha256Hex([]byte(backupXML)),
		PreRestoreBackupDir: restoreBackups,
		AuditDir:            auditDir,
		Now:                 time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Restored {
		t.Fatal("expected restored=true")
	}
	current, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != backupXML {
		t.Fatal("target was not restored from backup")
	}
	preRestoreFiles, err := os.ReadDir(restoreBackups)
	if err != nil {
		t.Fatal(err)
	}
	if len(preRestoreFiles) != 1 {
		t.Fatalf("expected one pre-restore backup, got %d", len(preRestoreFiles))
	}
	auditFiles, err := os.ReadDir(auditDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(auditFiles) != 1 {
		t.Fatalf("expected one audit file, got %d", len(auditFiles))
	}
}
