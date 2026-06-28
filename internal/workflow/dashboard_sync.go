package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/lifecycle"
	"unraid-ai-manager/internal/planner"
)

const (
	DashboardSyncKind = "dashboard-sync"

	RecreateModeChanged = "changed"
	RecreateModeAll     = "all"
	RecreateModeNone    = "none"
)

type DashboardSyncOptions struct {
	RecreateMode string
}

type DashboardSyncPlan struct {
	Kind          string                 `json:"kind"`
	Provider      string                 `json:"provider"`
	Adapter       string                 `json:"adapter"`
	WriteEnabled  bool                   `json:"write_enabled"`
	RecreateMode  string                 `json:"recreate_mode"`
	DashboardPlan planner.DashboardPlan  `json:"dashboard_plan"`
	RecreatePlan  lifecycle.RecreatePlan `json:"recreate_plan"`
	Entries       []DashboardSyncEntry   `json:"entries"`
	PlanHash      string                 `json:"plan_hash"`
}

type DashboardSyncEntry struct {
	Container          string   `json:"container"`
	SourcePath         string   `json:"source_path"`
	URL                string   `json:"url,omitempty"`
	State              string   `json:"state,omitempty"`
	DashboardChanged   bool     `json:"dashboard_changed"`
	RecreatePlanned    bool     `json:"recreate_planned"`
	RecreatePlanReason string   `json:"recreate_plan_reason,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

func NormalizeRecreateMode(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return RecreateModeChanged, nil
	}
	switch value {
	case RecreateModeChanged, RecreateModeAll, RecreateModeNone:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported recreate mode: %s", value)
	}
}

func BuildDashboardSyncPlan(templates []dockerxml.Template, runtime []dockerinspect.Container, dashboardPlan planner.DashboardPlan, options DashboardSyncOptions) (DashboardSyncPlan, error) {
	if dashboardPlan.Kind != "dashboard-config" {
		return DashboardSyncPlan{}, fmt.Errorf("unsupported dashboard plan kind: %s", dashboardPlan.Kind)
	}
	provider, err := planner.NormalizeDashboardProvider(dashboardPlan.Provider)
	if err != nil {
		return DashboardSyncPlan{}, err
	}
	recreateMode, err := NormalizeRecreateMode(options.RecreateMode)
	if err != nil {
		return DashboardSyncPlan{}, err
	}

	plan := DashboardSyncPlan{
		Kind:          DashboardSyncKind,
		Provider:      provider,
		Adapter:       dashboardPlan.Adapter,
		WriteEnabled:  dashboardPlan.WriteEnabled,
		RecreateMode:  recreateMode,
		DashboardPlan: dashboardPlan,
	}

	recreateNames := map[string]bool{}
	changedByName := map[string]bool{}
	for _, entry := range dashboardPlan.Entries {
		changed := DashboardEntryChanged(entry)
		key := strings.ToLower(entry.Container)
		changedByName[key] = changed
		if recreateMode == RecreateModeAll || (recreateMode == RecreateModeChanged && changed) {
			recreateNames[key] = true
		}
	}

	if recreateMode != RecreateModeNone && len(recreateNames) > 0 {
		recreatePlan := lifecycle.BuildRecreatePlan(templates, runtime, lifecycle.Options{
			IncludeAll: true,
			Names:      recreateNames,
		})
		for index := range recreatePlan.Entries {
			key := strings.ToLower(recreatePlan.Entries[index].Container)
			reason := "dashboard XML template will change; DockerMan recreate is required for runtime configuration to receive updated dashboard settings"
			if !changedByName[key] && recreateMode == RecreateModeAll {
				reason = "recreate requested for all dashboard entries"
			}
			recreatePlan.Entries[index].Reasons = prependReason(reason, recreatePlan.Entries[index].Reasons)
			if recreatePlan.Entries[index].Metadata == nil {
				recreatePlan.Entries[index].Metadata = map[string]string{}
			}
			recreatePlan.Entries[index].Metadata["operation"] = DashboardSyncKind
			recreatePlan.Entries[index].Metadata["dashboard_provider"] = provider
		}
		plan.RecreatePlan = lifecycle.FinalizeRecreatePlan(recreatePlan)
	}

	recreateByName := map[string]lifecycle.RecreateEntry{}
	for _, entry := range plan.RecreatePlan.Entries {
		recreateByName[strings.ToLower(entry.Container)] = entry
	}
	runtimeByName := dockerinspect.IndexByName(runtime)
	for _, dashboardEntry := range dashboardPlan.Entries {
		key := strings.ToLower(dashboardEntry.Container)
		syncEntry := DashboardSyncEntry{
			Container:        dashboardEntry.Container,
			SourcePath:       dashboardEntry.SourcePath,
			URL:              dashboardEntry.URL.URL,
			DashboardChanged: changedByName[key],
			Warnings:         append([]string{}, dashboardEntry.Warnings...),
		}
		if container, ok := runtimeByName[key]; ok {
			syncEntry.State = container.State
		}
		if recreateEntry, ok := recreateByName[key]; ok {
			syncEntry.RecreatePlanned = true
			if len(recreateEntry.Reasons) > 0 {
				syncEntry.RecreatePlanReason = recreateEntry.Reasons[0]
			}
		} else if recreateNames[key] {
			syncEntry.Warnings = append(syncEntry.Warnings, "Runtime container was not found; recreate cannot be planned for this dashboard change.")
		}
		plan.Entries = append(plan.Entries, syncEntry)
	}

	plan.PlanHash = hashDashboardSyncPlan(plan)
	return plan, nil
}

func DashboardEntryChanged(entry planner.DashboardEntry) bool {
	for _, change := range entry.TargetChanges {
		if change.Action != "unchanged" {
			return true
		}
	}
	return false
}

func prependReason(reason string, reasons []string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append([]string{reason}, reasons...)
}

func hashDashboardSyncPlan(plan DashboardSyncPlan) string {
	plan.PlanHash = ""
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
