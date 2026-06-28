package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"unraid-ai-manager/internal/planner"
	"unraid-ai-manager/internal/xmlpatch"
)

type TZApplyOptions struct {
	ConfirmPlanHash string
	BackupDir       string
	AuditDir        string
	Now             time.Time
}

type TZApplyReport struct {
	PlanHash   string          `json:"plan_hash"`
	StartedAt  string          `json:"started_at"`
	FinishedAt string          `json:"finished_at"`
	BackupDir  string          `json:"backup_dir"`
	AuditDir   string          `json:"audit_dir"`
	Results    []TZApplyResult `json:"results"`
}

type TZApplyResult struct {
	Container      string               `json:"container"`
	SourcePath     string               `json:"source_path"`
	Changed        bool                 `json:"changed"`
	OriginalSHA256 string               `json:"original_sha256"`
	ModifiedSHA256 string               `json:"modified_sha256,omitempty"`
	BackupPath     string               `json:"backup_path,omitempty"`
	Operations     []xmlpatch.Operation `json:"operations,omitempty"`
}

func ReadTZPlanFile(path string) (planner.TZPlan, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return planner.TZPlan{}, err
	}

	var plan planner.TZPlan
	if err := json.Unmarshal(payload, &plan); err != nil {
		return planner.TZPlan{}, err
	}
	if plan.Kind != "template-tz" {
		return planner.TZPlan{}, fmt.Errorf("plan file does not contain a TZ plan")
	}
	return plan, nil
}

func ApplyTZPlan(plan planner.TZPlan, options TZApplyOptions) (TZApplyReport, error) {
	if plan.Kind != "template-tz" {
		return TZApplyReport{}, fmt.Errorf("unsupported plan kind: %s", plan.Kind)
	}
	if plan.PlanHash == "" {
		return TZApplyReport{}, fmt.Errorf("plan hash is empty")
	}
	if options.ConfirmPlanHash == "" {
		return TZApplyReport{}, fmt.Errorf("--confirm-plan-hash is required")
	}
	if options.ConfirmPlanHash != plan.PlanHash {
		return TZApplyReport{}, fmt.Errorf("confirmation hash mismatch: got %s, expected %s", options.ConfirmPlanHash, plan.PlanHash)
	}
	if options.BackupDir == "" {
		return TZApplyReport{}, fmt.Errorf("--backup-dir is required")
	}
	if options.AuditDir == "" {
		return TZApplyReport{}, fmt.Errorf("--audit-dir is required")
	}

	start := options.Now
	if start.IsZero() {
		start = time.Now().UTC()
	}
	backupDir, err := filepath.Abs(options.BackupDir)
	if err != nil {
		return TZApplyReport{}, err
	}
	auditDir, err := filepath.Abs(options.AuditDir)
	if err != nil {
		return TZApplyReport{}, err
	}
	if err := validateTZBackupAndAuditDirs(plan, backupDir, auditDir); err != nil {
		return TZApplyReport{}, err
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return TZApplyReport{}, fmt.Errorf("create backup dir: %w", err)
	}
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		return TZApplyReport{}, fmt.Errorf("create audit dir: %w", err)
	}

	report := TZApplyReport{
		PlanHash:  plan.PlanHash,
		StartedAt: start.Format(time.RFC3339),
		BackupDir: backupDir,
		AuditDir:  auditDir,
	}
	for _, entry := range plan.Entries {
		result, err := applyTZEntry(entry, backupDir, start)
		if err != nil {
			return report, err
		}
		report.Results = append(report.Results, result)
	}
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeTZAudit(report, auditDir, start); err != nil {
		return report, err
	}
	return report, nil
}

func applyTZEntry(entry planner.TZEntry, backupDir string, timestamp time.Time) (TZApplyResult, error) {
	originalBytes, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return TZApplyResult{}, fmt.Errorf("read %s: %w", entry.SourcePath, err)
	}
	patch, err := xmlpatch.ApplyVariable(string(originalBytes), "TZ", entry.ProposedValue, "TZ")
	if err != nil {
		return TZApplyResult{}, fmt.Errorf("patch %s: %w", entry.SourcePath, err)
	}
	result := TZApplyResult{
		Container:      entry.Container,
		SourcePath:     entry.SourcePath,
		Changed:        patch.Changed,
		OriginalSHA256: sha256Hex(originalBytes),
		Operations:     patch.Ops,
	}
	if !patch.Changed {
		return result, nil
	}

	backupPath := backupPathFor(backupDir, entry.SourcePath, result.OriginalSHA256, timestamp)
	if err := os.WriteFile(backupPath, originalBytes, 0o600); err != nil {
		return TZApplyResult{}, fmt.Errorf("write backup %s: %w", backupPath, err)
	}

	modifiedBytes := []byte(patch.Modified)
	result.ModifiedSHA256 = sha256Hex(modifiedBytes)
	result.BackupPath = backupPath
	info, err := os.Stat(entry.SourcePath)
	if err != nil {
		return TZApplyResult{}, fmt.Errorf("stat %s: %w", entry.SourcePath, err)
	}
	if err := replaceFile(entry.SourcePath, modifiedBytes, info.Mode()); err != nil {
		return TZApplyResult{}, err
	}
	return result, nil
}

func validateTZBackupAndAuditDirs(plan planner.TZPlan, backupDir string, auditDir string) error {
	for _, entry := range plan.Entries {
		sourceAbs, err := filepath.Abs(entry.SourcePath)
		if err != nil {
			return err
		}
		sourceDir := filepath.Dir(sourceAbs)
		if sameOrInside(backupDir, sourceDir) {
			return fmt.Errorf("backup dir must not be inside template dir %s", sourceDir)
		}
		if sameOrInside(auditDir, sourceDir) {
			return fmt.Errorf("audit dir must not be inside template dir %s", sourceDir)
		}
	}
	return nil
}

func writeTZAudit(report TZApplyReport, auditDir string, timestamp time.Time) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := filepath.Join(auditDir, fmt.Sprintf("%s_tz_%s.json", timestamp.UTC().Format("20060102T150405Z"), report.PlanHash))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write audit %s: %w", path, err)
	}
	return nil
}
