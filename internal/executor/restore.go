package executor

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"unraid-ai-manager/internal/dockerxml"
)

type RestoreXMLOptions struct {
	BackupPath          string
	TargetPath          string
	ConfirmBackupSHA256 string
	PreRestoreBackupDir string
	AuditDir            string
	Now                 time.Time
}

type RestoreXMLReport struct {
	StartedAt           string `json:"started_at"`
	FinishedAt          string `json:"finished_at"`
	BackupPath          string `json:"backup_path"`
	TargetPath          string `json:"target_path"`
	PreRestoreBackupDir string `json:"pre_restore_backup_dir"`
	AuditDir            string `json:"audit_dir"`
	BackupSHA256        string `json:"backup_sha256"`
	TargetBeforeSHA256  string `json:"target_before_sha256"`
	TargetAfterSHA256   string `json:"target_after_sha256"`
	PreRestoreBackup    string `json:"pre_restore_backup"`
	Restored            bool   `json:"restored"`
}

func RestoreXMLBackup(options RestoreXMLOptions) (RestoreXMLReport, error) {
	if options.BackupPath == "" {
		return RestoreXMLReport{}, fmt.Errorf("--backup is required")
	}
	if options.TargetPath == "" {
		return RestoreXMLReport{}, fmt.Errorf("--target is required")
	}
	if options.ConfirmBackupSHA256 == "" {
		return RestoreXMLReport{}, fmt.Errorf("--confirm-backup-sha256 is required")
	}
	if options.PreRestoreBackupDir == "" {
		return RestoreXMLReport{}, fmt.Errorf("--pre-restore-backup-dir is required")
	}
	if options.AuditDir == "" {
		return RestoreXMLReport{}, fmt.Errorf("--audit-dir is required")
	}

	start := options.Now
	if start.IsZero() {
		start = time.Now().UTC()
	}

	backupPath, err := filepath.Abs(options.BackupPath)
	if err != nil {
		return RestoreXMLReport{}, err
	}
	targetPath, err := filepath.Abs(options.TargetPath)
	if err != nil {
		return RestoreXMLReport{}, err
	}
	preRestoreBackupDir, err := filepath.Abs(options.PreRestoreBackupDir)
	if err != nil {
		return RestoreXMLReport{}, err
	}
	auditDir, err := filepath.Abs(options.AuditDir)
	if err != nil {
		return RestoreXMLReport{}, err
	}

	if backupPath == targetPath {
		return RestoreXMLReport{}, fmt.Errorf("backup and target must be different files")
	}
	targetDir := filepath.Dir(targetPath)
	if sameOrInside(preRestoreBackupDir, targetDir) {
		return RestoreXMLReport{}, fmt.Errorf("pre-restore backup dir must not be inside template dir %s", targetDir)
	}
	if sameOrInside(auditDir, targetDir) {
		return RestoreXMLReport{}, fmt.Errorf("audit dir must not be inside template dir %s", targetDir)
	}

	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		return RestoreXMLReport{}, fmt.Errorf("read backup %s: %w", backupPath, err)
	}
	if err := validateContainerXML(backupBytes); err != nil {
		return RestoreXMLReport{}, fmt.Errorf("backup XML is invalid: %w", err)
	}
	backupSHA := sha256Hex(backupBytes)
	if backupSHA != options.ConfirmBackupSHA256 {
		return RestoreXMLReport{}, fmt.Errorf("backup SHA256 mismatch: got %s, expected confirmation %s", backupSHA, options.ConfirmBackupSHA256)
	}

	targetBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return RestoreXMLReport{}, fmt.Errorf("read target %s: %w", targetPath, err)
	}
	if err := validateContainerXML(targetBytes); err != nil {
		return RestoreXMLReport{}, fmt.Errorf("target XML is invalid: %w", err)
	}
	targetBeforeSHA := sha256Hex(targetBytes)

	if err := os.MkdirAll(preRestoreBackupDir, 0o700); err != nil {
		return RestoreXMLReport{}, fmt.Errorf("create pre-restore backup dir: %w", err)
	}
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		return RestoreXMLReport{}, fmt.Errorf("create audit dir: %w", err)
	}

	preRestorePath := backupPathFor(preRestoreBackupDir, targetPath, targetBeforeSHA, start)
	if err := os.WriteFile(preRestorePath, targetBytes, 0o600); err != nil {
		return RestoreXMLReport{}, fmt.Errorf("write pre-restore backup %s: %w", preRestorePath, err)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return RestoreXMLReport{}, fmt.Errorf("stat target %s: %w", targetPath, err)
	}
	if err := replaceFile(targetPath, backupBytes, info.Mode()); err != nil {
		return RestoreXMLReport{}, err
	}

	report := RestoreXMLReport{
		StartedAt:           start.Format(time.RFC3339),
		FinishedAt:          time.Now().UTC().Format(time.RFC3339),
		BackupPath:          backupPath,
		TargetPath:          targetPath,
		PreRestoreBackupDir: preRestoreBackupDir,
		AuditDir:            auditDir,
		BackupSHA256:        backupSHA,
		TargetBeforeSHA256:  targetBeforeSHA,
		TargetAfterSHA256:   sha256Hex(backupBytes),
		PreRestoreBackup:    preRestorePath,
		Restored:            true,
	}
	if err := writeRestoreAudit(report, auditDir, start); err != nil {
		return report, err
	}
	return report, nil
}

func validateContainerXML(payload []byte) error {
	var template dockerxml.Template
	if err := xml.Unmarshal(payload, &template); err != nil {
		return err
	}
	if template.XMLName.Local != "Container" {
		return fmt.Errorf("root element is %q, expected Container", template.XMLName.Local)
	}
	return nil
}

func writeRestoreAudit(report RestoreXMLReport, auditDir string, timestamp time.Time) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := filepath.Join(auditDir, fmt.Sprintf("%s_restore_%s.json", timestamp.UTC().Format("20060102T150405Z"), shortHash(report.BackupSHA256)))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write restore audit %s: %w", path, err)
	}
	return nil
}

func shortHash(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
