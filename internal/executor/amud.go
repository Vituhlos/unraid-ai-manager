package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"unraid-ai-manager/internal/planner"
	"unraid-ai-manager/internal/xmlpatch"
)

type AMUDApplyOptions struct {
	ConfirmPlanHash string
	BackupDir       string
	AuditDir        string
	Now             time.Time
}

type AMUDApplyReport struct {
	PlanHash   string            `json:"plan_hash"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	BackupDir  string            `json:"backup_dir"`
	AuditDir   string            `json:"audit_dir"`
	Results    []AMUDApplyResult `json:"results"`
}

type AMUDApplyResult struct {
	Container      string               `json:"container"`
	SourcePath     string               `json:"source_path"`
	Changed        bool                 `json:"changed"`
	OriginalSHA256 string               `json:"original_sha256"`
	ModifiedSHA256 string               `json:"modified_sha256,omitempty"`
	BackupPath     string               `json:"backup_path,omitempty"`
	Operations     []xmlpatch.Operation `json:"operations,omitempty"`
}

func ReadAMUDPlanFile(path string) (planner.AMUDPlan, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return planner.AMUDPlan{}, err
	}

	var wrapped struct {
		Plan planner.AMUDPlan `json:"plan"`
	}
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Plan.Kind != "" {
		return wrapped.Plan, nil
	}

	var plan planner.AMUDPlan
	if err := json.Unmarshal(payload, &plan); err != nil {
		return planner.AMUDPlan{}, err
	}
	if plan.Kind == "" {
		return planner.AMUDPlan{}, fmt.Errorf("plan file does not contain an AMUD plan")
	}
	return plan, nil
}

func ApplyAMUDPlan(plan planner.AMUDPlan, options AMUDApplyOptions) (AMUDApplyReport, error) {
	if plan.Kind != "amud-labels" {
		return AMUDApplyReport{}, fmt.Errorf("unsupported plan kind: %s", plan.Kind)
	}
	if plan.PlanHash == "" {
		return AMUDApplyReport{}, fmt.Errorf("plan hash is empty")
	}
	if options.ConfirmPlanHash == "" {
		return AMUDApplyReport{}, fmt.Errorf("--confirm-plan-hash is required")
	}
	if options.ConfirmPlanHash != plan.PlanHash {
		return AMUDApplyReport{}, fmt.Errorf("confirmation hash mismatch: got %s, expected %s", options.ConfirmPlanHash, plan.PlanHash)
	}
	if options.BackupDir == "" {
		return AMUDApplyReport{}, fmt.Errorf("--backup-dir is required")
	}
	if options.AuditDir == "" {
		return AMUDApplyReport{}, fmt.Errorf("--audit-dir is required")
	}

	start := options.Now
	if start.IsZero() {
		start = time.Now().UTC()
	}

	backupDir, err := filepath.Abs(options.BackupDir)
	if err != nil {
		return AMUDApplyReport{}, err
	}
	auditDir, err := filepath.Abs(options.AuditDir)
	if err != nil {
		return AMUDApplyReport{}, err
	}
	if err := validateBackupAndAuditDirs(plan, backupDir, auditDir); err != nil {
		return AMUDApplyReport{}, err
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return AMUDApplyReport{}, fmt.Errorf("create backup dir: %w", err)
	}
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		return AMUDApplyReport{}, fmt.Errorf("create audit dir: %w", err)
	}

	report := AMUDApplyReport{
		PlanHash:  plan.PlanHash,
		StartedAt: start.Format(time.RFC3339),
		BackupDir: backupDir,
		AuditDir:  auditDir,
	}

	for _, entry := range plan.Entries {
		result, err := applyAMUDEntry(entry, plan.PlanHash, backupDir, start)
		if err != nil {
			return report, err
		}
		report.Results = append(report.Results, result)
	}

	finished := time.Now().UTC()
	report.FinishedAt = finished.Format(time.RFC3339)
	if err := writeAudit(report, auditDir, start); err != nil {
		return report, err
	}
	return report, nil
}

func applyAMUDEntry(entry planner.AMUDEntry, planHash string, backupDir string, timestamp time.Time) (AMUDApplyResult, error) {
	originalBytes, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return AMUDApplyResult{}, fmt.Errorf("read %s: %w", entry.SourcePath, err)
	}
	original := string(originalBytes)
	originalSHA := sha256Hex(originalBytes)

	patch, err := xmlpatch.ApplyAMUDLabels(original, entry.ProposedLabels)
	if err != nil {
		return AMUDApplyResult{}, fmt.Errorf("patch %s: %w", entry.SourcePath, err)
	}

	result := AMUDApplyResult{
		Container:      entry.Container,
		SourcePath:     entry.SourcePath,
		Changed:        patch.Changed,
		OriginalSHA256: originalSHA,
		Operations:     patch.Ops,
	}
	if !patch.Changed {
		return result, nil
	}

	backupPath := backupPathFor(backupDir, entry.SourcePath, originalSHA, timestamp)
	if err := os.WriteFile(backupPath, originalBytes, 0o600); err != nil {
		return AMUDApplyResult{}, fmt.Errorf("write backup %s: %w", backupPath, err)
	}

	modifiedBytes := []byte(patch.Modified)
	result.ModifiedSHA256 = sha256Hex(modifiedBytes)
	result.BackupPath = backupPath

	info, err := os.Stat(entry.SourcePath)
	if err != nil {
		return AMUDApplyResult{}, fmt.Errorf("stat %s: %w", entry.SourcePath, err)
	}
	if err := replaceFile(entry.SourcePath, modifiedBytes, info.Mode()); err != nil {
		return AMUDApplyResult{}, err
	}
	return result, nil
}

func validateBackupAndAuditDirs(plan planner.AMUDPlan, backupDir string, auditDir string) error {
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

func sameOrInside(child string, parent string) bool {
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func backupPathFor(backupDir string, sourcePath string, originalSHA string, timestamp time.Time) string {
	base := filepath.Base(sourcePath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = safeFileName(base)
	shortHash := originalSHA
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	name := fmt.Sprintf("%s_%s_%s.xml", timestamp.UTC().Format("20060102T150405Z"), base, shortHash)
	return filepath.Join(backupDir, name)
}

func safeFileName(value string) string {
	replacer := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(value)
}

func replaceFile(path string, payload []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".unraid-ai-manager-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file near %s: %w", path, err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp file %s: %w", tempPath, err)
	}
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return fmt.Errorf("chmod temp file %s: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tempPath, err)
	}

	if runtime.GOOS == "windows" {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove original before replace on windows %s: %w", path, err)
		}
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	cleanup = false
	return nil
}

func writeAudit(report AMUDApplyReport, auditDir string, timestamp time.Time) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := filepath.Join(auditDir, fmt.Sprintf("%s_amud_%s.json", timestamp.UTC().Format("20060102T150405Z"), report.PlanHash))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write audit %s: %w", path, err)
	}
	return nil
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
