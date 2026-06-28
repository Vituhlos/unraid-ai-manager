package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"unraid-ai-manager/internal/planner"
	"unraid-ai-manager/internal/xmlpatch"
)

type DashboardApplyOptions struct {
	ConfirmPlanHash string
	BackupDir       string
	AuditDir        string
	Now             time.Time
}

type DashboardApplyReport struct {
	Provider   string                 `json:"provider"`
	Adapter    string                 `json:"adapter"`
	PlanHash   string                 `json:"plan_hash"`
	StartedAt  string                 `json:"started_at"`
	FinishedAt string                 `json:"finished_at"`
	BackupDir  string                 `json:"backup_dir"`
	AuditDir   string                 `json:"audit_dir"`
	Results    []DashboardApplyResult `json:"results"`
}

type DashboardApplyResult struct {
	Container      string               `json:"container"`
	SourcePath     string               `json:"source_path"`
	Changed        bool                 `json:"changed"`
	OriginalSHA256 string               `json:"original_sha256"`
	ModifiedSHA256 string               `json:"modified_sha256,omitempty"`
	BackupPath     string               `json:"backup_path,omitempty"`
	Operations     []xmlpatch.Operation `json:"operations,omitempty"`
}

func ReadDashboardPlanFile(path string) (planner.DashboardPlan, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return planner.DashboardPlan{}, err
	}

	var wrapped struct {
		Plan planner.DashboardPlan `json:"plan"`
	}
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Plan.Kind != "" {
		return wrapped.Plan, nil
	}

	var plan planner.DashboardPlan
	if err := json.Unmarshal(payload, &plan); err != nil {
		return planner.DashboardPlan{}, err
	}
	if plan.Kind == "" {
		return planner.DashboardPlan{}, fmt.Errorf("plan file does not contain a dashboard plan")
	}
	return plan, nil
}

func ApplyDashboardPlan(plan planner.DashboardPlan, options DashboardApplyOptions) (DashboardApplyReport, error) {
	if plan.Kind != "dashboard-config" {
		return DashboardApplyReport{}, fmt.Errorf("unsupported plan kind: %s", plan.Kind)
	}
	if plan.PlanHash == "" {
		return DashboardApplyReport{}, fmt.Errorf("plan hash is empty")
	}
	if options.ConfirmPlanHash == "" {
		return DashboardApplyReport{}, fmt.Errorf("--confirm-plan-hash is required")
	}
	if options.ConfirmPlanHash != plan.PlanHash {
		return DashboardApplyReport{}, fmt.Errorf("confirmation hash mismatch: got %s, expected %s", options.ConfirmPlanHash, plan.PlanHash)
	}

	provider, err := planner.NormalizeDashboardProvider(plan.Provider)
	if err != nil {
		return DashboardApplyReport{}, err
	}

	switch provider {
	case planner.DashboardProviderAMUD:
		plan.Provider = provider
		amudPlan, err := planner.AMUDPlanFromDashboardPlan(plan)
		if err != nil {
			return DashboardApplyReport{}, err
		}
		amudReport, err := ApplyAMUDPlan(amudPlan, AMUDApplyOptions{
			ConfirmPlanHash: options.ConfirmPlanHash,
			BackupDir:       options.BackupDir,
			AuditDir:        options.AuditDir,
			Now:             options.Now,
		})
		if err != nil {
			return DashboardApplyReport{}, err
		}
		return dashboardReportFromAMUD(plan, amudReport), nil
	default:
		return DashboardApplyReport{}, fmt.Errorf("unsupported dashboard provider: %s", plan.Provider)
	}
}

func dashboardReportFromAMUD(plan planner.DashboardPlan, report AMUDApplyReport) DashboardApplyReport {
	results := make([]DashboardApplyResult, 0, len(report.Results))
	for _, result := range report.Results {
		results = append(results, DashboardApplyResult{
			Container:      result.Container,
			SourcePath:     result.SourcePath,
			Changed:        result.Changed,
			OriginalSHA256: result.OriginalSHA256,
			ModifiedSHA256: result.ModifiedSHA256,
			BackupPath:     result.BackupPath,
			Operations:     result.Operations,
		})
	}
	return DashboardApplyReport{
		Provider:   plan.Provider,
		Adapter:    plan.Adapter,
		PlanHash:   report.PlanHash,
		StartedAt:  report.StartedAt,
		FinishedAt: report.FinishedAt,
		BackupDir:  report.BackupDir,
		AuditDir:   report.AuditDir,
		Results:    results,
	}
}
