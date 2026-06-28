package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/lifecycle"
)

const DefaultDockerManRebuildScript = "/usr/local/emhttp/plugins/dynamix.docker.manager/scripts/rebuild_container"

var safeContainerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type DockerRuntimeController interface {
	InspectContainer(ctx context.Context, id string) (dockerinspect.Container, error)
	StartContainer(ctx context.Context, id string) error
}

type RebuildRunner interface {
	Rebuild(ctx context.Context, scriptPath string, container string) (string, error)
}

type DockerManRebuildRunner struct{}

type RecreateApplyOptions struct {
	ConfirmPlanHash string
	AuditDir        string
	RebuildScript   string
	Timeout         time.Duration
	Now             time.Time
	Runtime         DockerRuntimeController
	Runner          RebuildRunner
}

type RecreateApplyReport struct {
	PlanHash      string                `json:"plan_hash"`
	StartedAt     string                `json:"started_at"`
	FinishedAt    string                `json:"finished_at"`
	AuditDir      string                `json:"audit_dir"`
	RebuildScript string                `json:"rebuild_script"`
	OK            bool                  `json:"ok"`
	FailureCount  int                   `json:"failure_count"`
	Results       []RecreateApplyResult `json:"results"`
}

type RecreateApplyResult struct {
	Container         string            `json:"container"`
	TemplatePath      string            `json:"template_path"`
	WasRunning        bool              `json:"was_running"`
	StateBefore       string            `json:"state_before,omitempty"`
	StateAfter        string            `json:"state_after,omitempty"`
	Rebuilt           bool              `json:"rebuilt"`
	StartedAfter      bool              `json:"started_after"`
	RuntimeAMUDLabels map[string]string `json:"runtime_amud_labels,omitempty"`
	Output            string            `json:"output,omitempty"`
	Error             string            `json:"error,omitempty"`
}

func (DockerManRebuildRunner) Rebuild(ctx context.Context, scriptPath string, container string) (string, error) {
	command := exec.CommandContext(ctx, scriptPath, container)
	output, err := command.CombinedOutput()
	return string(output), err
}

func ReadRecreatePlanFile(path string) (lifecycle.RecreatePlan, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return lifecycle.RecreatePlan{}, err
	}

	var wrapped struct {
		Plan lifecycle.RecreatePlan `json:"plan"`
	}
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Plan.Kind != "" {
		return wrapped.Plan, nil
	}

	var plan lifecycle.RecreatePlan
	if err := json.Unmarshal(payload, &plan); err != nil {
		return lifecycle.RecreatePlan{}, err
	}
	if plan.Kind == "" {
		return lifecycle.RecreatePlan{}, errors.New("plan file does not contain a recreate plan")
	}
	return plan, nil
}

func ApplyRecreatePlan(ctx context.Context, plan lifecycle.RecreatePlan, options RecreateApplyOptions) (RecreateApplyReport, error) {
	if plan.Kind != "docker-recreate" {
		return RecreateApplyReport{}, fmt.Errorf("unsupported plan kind: %s", plan.Kind)
	}
	if plan.PlanHash == "" {
		return RecreateApplyReport{}, errors.New("plan hash is empty")
	}
	if options.ConfirmPlanHash == "" {
		return RecreateApplyReport{}, errors.New("--confirm-plan-hash is required")
	}
	if options.ConfirmPlanHash != plan.PlanHash {
		return RecreateApplyReport{}, fmt.Errorf("confirmation hash mismatch: got %s, expected %s", options.ConfirmPlanHash, plan.PlanHash)
	}
	if options.AuditDir == "" {
		return RecreateApplyReport{}, errors.New("--audit-dir is required")
	}

	rebuildScript := options.RebuildScript
	if rebuildScript == "" {
		rebuildScript = DefaultDockerManRebuildScript
	}
	if err := validateRebuildScript(rebuildScript, options.Runner == nil); err != nil {
		return RecreateApplyReport{}, err
	}

	runner := options.Runner
	if runner == nil {
		runner = DockerManRebuildRunner{}
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	start := options.Now
	if start.IsZero() {
		start = time.Now().UTC()
	}
	auditDir, err := filepath.Abs(options.AuditDir)
	if err != nil {
		return RecreateApplyReport{}, err
	}
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		return RecreateApplyReport{}, fmt.Errorf("create audit dir: %w", err)
	}

	report := RecreateApplyReport{
		PlanHash:      plan.PlanHash,
		StartedAt:     start.Format(time.RFC3339),
		AuditDir:      auditDir,
		RebuildScript: rebuildScript,
		OK:            true,
		Results:       make([]RecreateApplyResult, 0, len(plan.Entries)),
	}

	for _, entry := range plan.Entries {
		result := applyRecreateEntry(ctx, entry, rebuildScript, timeout, options.Runtime, runner)
		if result.Error != "" {
			report.OK = false
			report.FailureCount++
		}
		report.Results = append(report.Results, result)
	}

	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeRecreateAudit(report, auditDir, start); err != nil {
		return report, err
	}
	return report, nil
}

func applyRecreateEntry(ctx context.Context, entry lifecycle.RecreateEntry, rebuildScript string, timeout time.Duration, runtime DockerRuntimeController, runner RebuildRunner) RecreateApplyResult {
	result := RecreateApplyResult{
		Container:    entry.Container,
		TemplatePath: entry.TemplatePath,
	}
	if err := validateContainerName(entry.Container); err != nil {
		result.Error = err.Error()
		return result
	}

	if runtime != nil {
		before, err := runtime.InspectContainer(ctx, entry.Container)
		if err != nil {
			result.Error = fmt.Sprintf("inspect before rebuild: %v", err)
			return result
		}
		result.StateBefore = before.State
		result.WasRunning = strings.EqualFold(before.State, "running")
	}

	rebuildCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := runner.Rebuild(rebuildCtx, rebuildScript, entry.Container)
	result.Output = truncateOutput(output)
	if err != nil {
		if errors.Is(rebuildCtx.Err(), context.DeadlineExceeded) {
			result.Error = fmt.Sprintf("DockerMan rebuild timed out after %s", timeout)
			return result
		}
		result.Error = fmt.Sprintf("DockerMan rebuild failed: %v", err)
		return result
	}
	result.Rebuilt = true

	if runtime == nil {
		return result
	}

	after, err := runtime.InspectContainer(ctx, entry.Container)
	if err != nil {
		result.Error = fmt.Sprintf("inspect after rebuild: %v", err)
		return result
	}
	result.StateAfter = after.State
	result.RuntimeAMUDLabels = filterAMUDLabels(after.Labels)

	if result.WasRunning && !strings.EqualFold(after.State, "running") {
		if err := runtime.StartContainer(ctx, entry.Container); err != nil {
			result.Error = fmt.Sprintf("container was running before rebuild but start failed: %v", err)
			return result
		}
		result.StartedAfter = true
		started, err := runtime.InspectContainer(ctx, entry.Container)
		if err != nil {
			result.Error = fmt.Sprintf("inspect after start: %v", err)
			return result
		}
		result.StateAfter = started.State
		result.RuntimeAMUDLabels = filterAMUDLabels(started.Labels)
	}

	return result
}

func validateRebuildScript(path string, requireExists bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("rebuild script path is empty")
	}
	clean := filepath.Clean(path)
	if filepath.Base(clean) != "rebuild_container" {
		return fmt.Errorf("unsupported rebuild script %s: expected rebuild_container", path)
	}
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("rebuild script must be an absolute path: %s", path)
	}
	if requireExists {
		info, err := os.Stat(clean)
		if err != nil {
			return fmt.Errorf("rebuild script is not available: %w", err)
		}
		if info.IsDir() {
			return fmt.Errorf("rebuild script is a directory: %s", clean)
		}
	}
	return nil
}

func validateContainerName(name string) error {
	if !safeContainerNamePattern.MatchString(name) {
		return fmt.Errorf("unsafe container name %q", name)
	}
	return nil
}

func filterAMUDLabels(labels map[string]string) map[string]string {
	filtered := map[string]string{}
	for key, value := range labels {
		if strings.HasPrefix(key, "amud.") {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func truncateOutput(output string) string {
	const maxOutput = 20_000
	output = strings.TrimSpace(output)
	if len(output) <= maxOutput {
		return output
	}
	return output[:maxOutput] + "\n... output truncated ..."
}

func writeRecreateAudit(report RecreateApplyReport, auditDir string, timestamp time.Time) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := filepath.Join(auditDir, fmt.Sprintf("%s_recreate_%s.json", timestamp.UTC().Format("20060102T150405Z"), report.PlanHash))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write audit %s: %w", path, err)
	}
	return nil
}
