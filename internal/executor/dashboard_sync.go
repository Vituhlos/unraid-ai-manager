package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"unraid-ai-manager/internal/workflow"
)

type DashboardSyncApplyOptions struct {
	ConfirmPlanHash string
	BackupDir       string
	AuditDir        string
	Now             time.Time
	Runtime         DockerRuntimeController
	Runner          RebuildRunner
	RebuildScript   string
	Timeout         time.Duration
}

type DashboardSyncApplyReport struct {
	PlanHash        string                `json:"plan_hash"`
	StartedAt       string                `json:"started_at"`
	FinishedAt      string                `json:"finished_at"`
	OK              bool                  `json:"ok"`
	DashboardReport DashboardApplyReport  `json:"dashboard_report"`
	RecreateReport  *RecreateApplyReport  `json:"recreate_report,omitempty"`
	Verification    []DashboardSyncVerify `json:"verification,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
}

type DashboardSyncVerify struct {
	Container      string            `json:"container"`
	State          string            `json:"state,omitempty"`
	ExpectedLabels map[string]string `json:"expected_labels,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Error          string            `json:"error,omitempty"`
}

func ReadDashboardSyncPlanFile(path string) (workflow.DashboardSyncPlan, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return workflow.DashboardSyncPlan{}, err
	}

	var wrapped struct {
		Plan workflow.DashboardSyncPlan `json:"plan"`
	}
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Plan.Kind != "" {
		return wrapped.Plan, nil
	}

	var plan workflow.DashboardSyncPlan
	if err := json.Unmarshal(payload, &plan); err != nil {
		return workflow.DashboardSyncPlan{}, err
	}
	if plan.Kind == "" {
		return workflow.DashboardSyncPlan{}, errors.New("plan file does not contain a dashboard sync plan")
	}
	return plan, nil
}

func ApplyDashboardSyncPlan(ctx context.Context, plan workflow.DashboardSyncPlan, options DashboardSyncApplyOptions) (DashboardSyncApplyReport, error) {
	if plan.Kind != workflow.DashboardSyncKind {
		return DashboardSyncApplyReport{}, fmt.Errorf("unsupported plan kind: %s", plan.Kind)
	}
	if plan.PlanHash == "" {
		return DashboardSyncApplyReport{}, errors.New("plan hash is empty")
	}
	if options.ConfirmPlanHash == "" {
		return DashboardSyncApplyReport{}, errors.New("--confirm-plan-hash is required")
	}
	if options.ConfirmPlanHash != plan.PlanHash {
		return DashboardSyncApplyReport{}, fmt.Errorf("confirmation hash mismatch: got %s, expected %s", options.ConfirmPlanHash, plan.PlanHash)
	}

	start := options.Now
	if start.IsZero() {
		start = time.Now().UTC()
	}
	report := DashboardSyncApplyReport{
		PlanHash:  plan.PlanHash,
		StartedAt: start.Format(time.RFC3339),
		OK:        true,
	}

	dashboardReport, err := ApplyDashboardPlan(plan.DashboardPlan, DashboardApplyOptions{
		ConfirmPlanHash: plan.DashboardPlan.PlanHash,
		BackupDir:       options.BackupDir,
		AuditDir:        options.AuditDir,
		Now:             start,
	})
	if err != nil {
		report.OK = false
		return report, err
	}
	report.DashboardReport = dashboardReport

	if len(plan.RecreatePlan.Entries) > 0 {
		if options.Runtime == nil {
			report.OK = false
			return report, errors.New("runtime controller is required when dashboard sync includes recreate entries")
		}
		recreateReport, err := ApplyRecreatePlan(ctx, plan.RecreatePlan, RecreateApplyOptions{
			ConfirmPlanHash: plan.RecreatePlan.PlanHash,
			AuditDir:        options.AuditDir,
			RebuildScript:   options.RebuildScript,
			Timeout:         options.Timeout,
			Now:             start,
			Runtime:         options.Runtime,
			Runner:          options.Runner,
		})
		report.RecreateReport = &recreateReport
		if err != nil {
			report.OK = false
			return report, err
		}
		if !recreateReport.OK {
			report.OK = false
		}
	} else {
		report.Warnings = append(report.Warnings, "No recreate entries were planned; runtime containers may already match or no dashboard XML changes were needed.")
	}

	report.Verification = verifyDashboardSyncRuntime(ctx, plan, options.Runtime)
	for _, item := range report.Verification {
		if item.Error != "" {
			report.OK = false
			break
		}
	}
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeDashboardSyncAudit(report, options.AuditDir, start); err != nil {
		return report, err
	}
	return report, nil
}

func verifyDashboardSyncRuntime(ctx context.Context, plan workflow.DashboardSyncPlan, runtime DockerRuntimeController) []DashboardSyncVerify {
	if runtime == nil {
		return nil
	}
	expectedByContainer := map[string]map[string]string{}
	for _, entry := range plan.DashboardPlan.Entries {
		expected := map[string]string{}
		for key, value := range entry.ProposedState {
			expected[key] = value
		}
		expectedByContainer[entry.Container] = expected
	}
	var records []DashboardSyncVerify
	for _, entry := range plan.Entries {
		if !entry.RecreatePlanned {
			continue
		}
		record := DashboardSyncVerify{
			Container:      entry.Container,
			ExpectedLabels: expectedByContainer[entry.Container],
		}
		container, err := runtime.InspectContainer(ctx, entry.Container)
		if err != nil {
			record.Error = err.Error()
			records = append(records, record)
			continue
		}
		record.State = container.State
		record.Labels = filterAMUDLabels(container.Labels)
		if mismatch := dashboardLabelMismatch(record.ExpectedLabels, record.Labels); mismatch != "" {
			record.Error = mismatch
		}
		records = append(records, record)
	}
	return records
}

func dashboardLabelMismatch(expected map[string]string, actual map[string]string) string {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok {
			return fmt.Sprintf("runtime label %s is missing", key)
		}
		if actualValue != expectedValue {
			return fmt.Sprintf("runtime label %s mismatch: expected %q, got %q", key, expectedValue, actualValue)
		}
	}
	return ""
}

func writeDashboardSyncAudit(report DashboardSyncApplyReport, auditDir string, timestamp time.Time) error {
	if auditDir == "" {
		return errors.New("--audit-dir is required")
	}
	auditDir, err := filepath.Abs(auditDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		return fmt.Errorf("create audit dir: %w", err)
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := filepath.Join(auditDir, fmt.Sprintf("%s_dashboard_sync_%s.json", timestamp.UTC().Format("20060102T150405Z"), report.PlanHash))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write audit %s: %w", path, err)
	}
	return nil
}
