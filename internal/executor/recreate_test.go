package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/lifecycle"
)

type fakeRebuildRunner struct {
	calls []string
	err   error
}

func (f *fakeRebuildRunner) Rebuild(_ context.Context, _ string, container string) (string, error) {
	f.calls = append(f.calls, container)
	if f.err != nil {
		return "failed", f.err
	}
	return "rebuilt " + container, nil
}

type fakeRuntimeController struct {
	state      string
	started    bool
	labels     map[string]string
	inspectErr error
	startErr   error
}

func (f *fakeRuntimeController) InspectContainer(_ context.Context, _ string) (dockerinspect.Container, error) {
	if f.inspectErr != nil {
		return dockerinspect.Container{}, f.inspectErr
	}
	return dockerinspect.Container{
		State:  f.state,
		Labels: f.labels,
	}, nil
}

func (f *fakeRuntimeController) StartContainer(_ context.Context, _ string) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	f.state = "running"
	return nil
}

func TestApplyRecreatePlanRequiresConfirmHash(t *testing.T) {
	_, err := ApplyRecreatePlan(context.Background(), lifecycle.RecreatePlan{
		Kind:     "docker-recreate",
		PlanHash: "abc",
	}, RecreateApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "--confirm-plan-hash") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestApplyRecreatePlanUsesDockerManRebuildAndRestartsPreviouslyRunningContainer(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "rebuild_container")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	auditDir := filepath.Join(dir, "audit")
	runner := &fakeRebuildRunner{}
	runtime := &fakeRuntimeController{
		state: "exited",
		labels: map[string]string{
			"amud.enable": "true",
			"amud.url":    "http://192.0.2.10:7878",
			"other":       "ignored",
		},
	}

	inspectCount := 0
	runtimeWithStateTransition := runtimeControllerFunc{
		inspect: func(ctx context.Context, id string) (dockerinspect.Container, error) {
			inspectCount++
			if inspectCount == 1 {
				return dockerinspect.Container{State: "running"}, nil
			}
			return runtime.InspectContainer(ctx, id)
		},
		start: runtime.StartContainer,
	}

	report, err := ApplyRecreatePlan(context.Background(), lifecycle.RecreatePlan{
		Kind:     "docker-recreate",
		PlanHash: "abc123",
		Entries: []lifecycle.RecreateEntry{{
			Container:    "radarr",
			TemplatePath: "/boot/config/plugins/dockerMan/templates-user/my-radarr.xml",
		}},
	}, RecreateApplyOptions{
		ConfirmPlanHash: "abc123",
		AuditDir:        auditDir,
		RebuildScript:   script,
		Runner:          runner,
		Runtime:         runtimeWithStateTransition,
		Now:             time.Date(2026, 6, 28, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.FailureCount != 0 {
		t.Fatalf("expected successful report, got %#v", report)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "radarr" {
		t.Fatalf("unexpected rebuild calls: %#v", runner.calls)
	}
	if !runtime.started {
		t.Fatal("expected previously running container to be started after rebuild")
	}
	if len(report.Results) != 1 || !report.Results[0].Rebuilt || !report.Results[0].StartedAfter {
		t.Fatalf("unexpected result: %#v", report.Results)
	}
	if report.Results[0].RuntimeAMUDLabels["other"] != "" {
		t.Fatalf("expected non-AMUD labels to be filtered, got %#v", report.Results[0].RuntimeAMUDLabels)
	}
	audits, err := os.ReadDir(auditDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one audit file, got %d", len(audits))
	}
}

type runtimeControllerFunc struct {
	inspect func(context.Context, string) (dockerinspect.Container, error)
	start   func(context.Context, string) error
}

func (f runtimeControllerFunc) InspectContainer(ctx context.Context, id string) (dockerinspect.Container, error) {
	return f.inspect(ctx, id)
}

func (f runtimeControllerFunc) StartContainer(ctx context.Context, id string) error {
	return f.start(ctx, id)
}
