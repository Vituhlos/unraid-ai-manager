package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"unraid-ai-manager/internal/discovery"
	"unraid-ai-manager/internal/planner"
)

const DashboardIntegrationPlanKind = "dashboard-integrations"

type DashboardIntegrationPlan struct {
	Kind     string                      `json:"kind"`
	Provider string                      `json:"provider"`
	Adapter  string                      `json:"adapter"`
	Entries  []DashboardIntegrationEntry `json:"entries"`
	PlanHash string                      `json:"plan_hash"`
	Notes    []string                    `json:"notes,omitempty"`
}

type DashboardIntegrationEntry struct {
	Container       string                 `json:"container"`
	ServiceType     string                 `json:"service_type,omitempty"`
	DisplayName     string                 `json:"display_name"`
	URL             string                 `json:"url,omitempty"`
	Status          string                 `json:"status"`
	RequiredSecrets []IntegrationSecretUse `json:"required_secrets,omitempty"`
	OptionalSecrets []IntegrationSecretUse `json:"optional_secrets,omitempty"`
	Capabilities    []string               `json:"capabilities,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
}

type IntegrationSecretUse struct {
	Name     string `json:"name"`
	Ref      string `json:"ref,omitempty"`
	Found    bool   `json:"found"`
	Required bool   `json:"required"`
	Preview  string `json:"preview,omitempty"`
	Length   int    `json:"length,omitempty"`
}

func BuildDashboardIntegrationPlan(dashboardPlan planner.DashboardPlan, discoveryReport discovery.Report) DashboardIntegrationPlan {
	plan := DashboardIntegrationPlan{
		Kind:     DashboardIntegrationPlanKind,
		Provider: dashboardPlan.Provider,
		Adapter:  dashboardPlan.Adapter,
		Notes: []string{
			"Read-only integration plan. Secret values are represented by secret_ref and are not exposed.",
			"Provider-specific integration apply is intentionally not implemented yet.",
		},
	}
	discoveryByContainer := map[string]discovery.Record{}
	for _, record := range discoveryReport.Records {
		discoveryByContainer[strings.ToLower(record.Container)] = record
	}
	for _, dashboardEntry := range dashboardPlan.Entries {
		record := discoveryByContainer[strings.ToLower(dashboardEntry.Container)]
		entry := DashboardIntegrationEntry{
			Container:   dashboardEntry.Container,
			ServiceType: dashboardEntry.Service.IntegrationType,
			DisplayName: dashboardEntry.Service.DisplayName,
			URL:         dashboardEntry.URL.URL,
			Warnings:    append([]string{}, dashboardEntry.Warnings...),
		}
		entry.RequiredSecrets = requiredSecretsFor(entry.ServiceType, record)
		entry.OptionalSecrets = optionalSecretsFor(entry.ServiceType, record)
		entry.Capabilities = capabilitiesFor(entry.ServiceType)
		entry.Status = integrationStatus(entry.ServiceType, entry.RequiredSecrets)
		entry.Warnings = append(entry.Warnings, record.Warnings...)
		plan.Entries = append(plan.Entries, entry)
	}
	plan.PlanHash = hashDashboardIntegrationPlan(plan)
	return plan
}

func requiredSecretsFor(serviceType string, record discovery.Record) []IntegrationSecretUse {
	switch serviceType {
	case "radarr", "sonarr", "prowlarr", "lidarr", "readarr", "whisparr", "tautulli":
		return []IntegrationSecretUse{secretUse(record, "api_key", true)}
	case "plex":
		return []IntegrationSecretUse{secretUse(record, "plex_token", true)}
	default:
		return nil
	}
}

func optionalSecretsFor(serviceType string, record discovery.Record) []IntegrationSecretUse {
	switch serviceType {
	case "overseerr", "jellyseerr", "jellyfin", "emby", "qbittorrent", "sabnzbd", "grafana", "uptime_kuma":
		return []IntegrationSecretUse{secretUse(record, "api_key", false)}
	default:
		return nil
	}
}

func secretUse(record discovery.Record, name string, required bool) IntegrationSecretUse {
	use := IntegrationSecretUse{Name: name, Required: required}
	for _, secret := range record.Secrets {
		if secret.Name != name {
			continue
		}
		use.Ref = secret.Ref
		use.Found = secret.Found
		use.Preview = secret.Preview
		use.Length = secret.Length
		return use
	}
	return use
}

func capabilitiesFor(serviceType string) []string {
	switch serviceType {
	case "radarr", "sonarr", "prowlarr", "lidarr", "readarr", "whisparr":
		return []string{"api-health", "queue/status", "version"}
	case "tautulli":
		return []string{"api-health", "plex-stats"}
	case "plex":
		return []string{"token-auth", "server-info"}
	case "cloudflare_tunnel":
		return []string{"tunnel-route-discovery"}
	default:
		if serviceType == "" {
			return nil
		}
		return []string{"dashboard-link"}
	}
}

func integrationStatus(serviceType string, required []IntegrationSecretUse) string {
	if serviceType == "" {
		return "unsupported"
	}
	for _, secret := range required {
		if !secret.Found || secret.Ref == "" {
			return "missing-secret"
		}
	}
	if len(required) == 0 && len(capabilitiesFor(serviceType)) == 0 {
		return "unsupported"
	}
	return "ready"
}

func hashDashboardIntegrationPlan(plan DashboardIntegrationPlan) string {
	plan.PlanHash = ""
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
