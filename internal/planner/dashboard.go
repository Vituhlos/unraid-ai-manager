package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"unraid-ai-manager/internal/dockerxml"
)

const (
	DashboardProviderAMUD = "amud"
	DashboardAdapterAMUD  = "dockerman-labels"
)

type DashboardOptions struct {
	Provider         string
	LocalHost        string
	URLMode          string
	CloudflareDomain string
	CloudflareRoutes map[string]string
	Names            map[string]bool
	ExcludedNames    map[string]bool
	IncludePortOnly  bool
	RuntimeFilter    string
}

type DashboardPlan struct {
	Kind             string           `json:"kind"`
	Provider         string           `json:"provider"`
	Adapter          string           `json:"adapter"`
	WriteEnabled     bool             `json:"write_enabled"`
	URLMode          string           `json:"url_mode"`
	LocalHost        string           `json:"local_host"`
	CloudflareDomain string           `json:"cloudflare_domain,omitempty"`
	IncludePortOnly  bool             `json:"include_port_only,omitempty"`
	RuntimeFilter    string           `json:"runtime_filter"`
	Entries          []DashboardEntry `json:"entries"`
	PlanHash         string           `json:"plan_hash"`
}

type DashboardEntry struct {
	Container       string                     `json:"container"`
	SourcePath      string                     `json:"source_path"`
	Repository      string                     `json:"repository"`
	TemplateURL     string                     `json:"template_url"`
	Service         DashboardService           `json:"service"`
	WebDetection    WebDetection               `json:"web_detection"`
	URL             URLResult                  `json:"url"`
	Target          string                     `json:"target"`
	CurrentState    map[string]string          `json:"current_state"`
	ProposedState   map[string]string          `json:"proposed_state"`
	TargetChanges   []DashboardTargetChange    `json:"target_changes"`
	ProviderPayload map[string]json.RawMessage `json:"provider_payload,omitempty"`
	Warnings        []string                   `json:"warnings"`
}

type DashboardService struct {
	DisplayName     string   `json:"display_name"`
	Slug            string   `json:"slug"`
	IntegrationType string   `json:"integration_type,omitempty"`
	Category        string   `json:"category,omitempty"`
	Icon            string   `json:"icon,omitempty"`
	Confidence      string   `json:"confidence"`
	Sources         []string `json:"sources,omitempty"`
}

type DashboardTargetChange struct {
	Action     string  `json:"action"`
	TargetType string  `json:"target_type"`
	Key        string  `json:"key"`
	Current    *string `json:"current"`
	Proposed   string  `json:"proposed"`
	Secret     bool    `json:"secret"`
}

func NormalizeDashboardProvider(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return DashboardProviderAMUD, nil
	}
	switch value {
	case DashboardProviderAMUD, "amud-dashboard", "amud-labels":
		return DashboardProviderAMUD, nil
	default:
		return "", fmt.Errorf("unsupported dashboard provider: %s", value)
	}
}

func BuildDashboardPlan(templates []dockerxml.Template, options DashboardOptions) (DashboardPlan, error) {
	provider, err := NormalizeDashboardProvider(options.Provider)
	if err != nil {
		return DashboardPlan{}, err
	}

	switch provider {
	case DashboardProviderAMUD:
		amud := BuildAMUDPlan(templates, AMUDOptions{
			LocalHost:        options.LocalHost,
			URLMode:          options.URLMode,
			CloudflareDomain: options.CloudflareDomain,
			CloudflareRoutes: options.CloudflareRoutes,
			Names:            options.Names,
			ExcludedNames:    options.ExcludedNames,
			IncludePortOnly:  options.IncludePortOnly,
			RuntimeFilter:    options.RuntimeFilter,
		})
		return DashboardPlanFromAMUDPlan(amud), nil
	default:
		return DashboardPlan{}, fmt.Errorf("unsupported dashboard provider: %s", provider)
	}
}

func DashboardPlanFromAMUDPlan(amud AMUDPlan) DashboardPlan {
	plan := DashboardPlan{
		Kind:             "dashboard-config",
		Provider:         DashboardProviderAMUD,
		Adapter:          DashboardAdapterAMUD,
		WriteEnabled:     amud.WriteEnabled,
		URLMode:          amud.URLMode,
		LocalHost:        amud.LocalHost,
		CloudflareDomain: amud.CloudflareDomain,
		IncludePortOnly:  amud.IncludePortOnly,
		RuntimeFilter:    amud.RuntimeFilter,
		Entries:          make([]DashboardEntry, 0, len(amud.Entries)),
	}

	for _, entry := range amud.Entries {
		targetChanges := make([]DashboardTargetChange, 0, len(entry.LabelChanges))
		for _, change := range entry.LabelChanges {
			targetChanges = append(targetChanges, DashboardTargetChange{
				Action:     change.Action,
				TargetType: "docker_label",
				Key:        change.Key,
				Current:    change.Current,
				Proposed:   change.Proposed,
				Secret:     false,
			})
		}
		plan.Entries = append(plan.Entries, DashboardEntry{
			Container:     entry.Container,
			SourcePath:    entry.SourcePath,
			Repository:    entry.Repository,
			TemplateURL:   entry.TemplateURL,
			Service:       InferDashboardService(entry.Container, entry.Repository, entry.TemplateURL),
			WebDetection:  entry.WebDetection,
			URL:           entry.URL,
			Target:        "dockerman_xml",
			CurrentState:  entry.CurrentLabels,
			ProposedState: entry.ProposedLabels,
			TargetChanges: targetChanges,
			Warnings:      entry.Warnings,
		})
	}

	plan.PlanHash = hashDashboardPlan(plan)
	return plan
}

func AMUDPlanFromDashboardPlan(plan DashboardPlan) (AMUDPlan, error) {
	provider, err := NormalizeDashboardProvider(plan.Provider)
	if err != nil {
		return AMUDPlan{}, err
	}
	if plan.Kind != "dashboard-config" {
		return AMUDPlan{}, fmt.Errorf("unsupported dashboard plan kind: %s", plan.Kind)
	}
	if provider != DashboardProviderAMUD {
		return AMUDPlan{}, fmt.Errorf("unsupported dashboard provider: %s", plan.Provider)
	}
	if plan.Adapter != "" && plan.Adapter != DashboardAdapterAMUD {
		return AMUDPlan{}, fmt.Errorf("unsupported AMUD adapter: %s", plan.Adapter)
	}

	amud := AMUDPlan{
		Kind:             "amud-labels",
		WriteEnabled:     plan.WriteEnabled,
		URLMode:          plan.URLMode,
		LocalHost:        plan.LocalHost,
		CloudflareDomain: plan.CloudflareDomain,
		IncludePortOnly:  plan.IncludePortOnly,
		RuntimeFilter:    plan.RuntimeFilter,
		Entries:          make([]AMUDEntry, 0, len(plan.Entries)),
		PlanHash:         plan.PlanHash,
	}
	for _, entry := range plan.Entries {
		labels := dockerLabelState(entry.ProposedState)
		changes := make([]LabelChange, 0, len(entry.TargetChanges))
		for _, change := range entry.TargetChanges {
			if change.TargetType != "docker_label" {
				continue
			}
			changes = append(changes, LabelChange{
				Action:   change.Action,
				Key:      change.Key,
				Current:  change.Current,
				Proposed: change.Proposed,
			})
		}
		amud.Entries = append(amud.Entries, AMUDEntry{
			Container:      entry.Container,
			SourcePath:     entry.SourcePath,
			Repository:     entry.Repository,
			TemplateURL:    entry.TemplateURL,
			WebDetection:   entry.WebDetection,
			URL:            entry.URL,
			CurrentLabels:  dockerLabelState(entry.CurrentState),
			ProposedLabels: labels,
			LabelChanges:   changes,
			Warnings:       entry.Warnings,
		})
	}
	return amud, nil
}

func InferDashboardService(name string, repository string, templateURL string) DashboardService {
	slug := InferRouteKey(name)
	signal := strings.ToLower(strings.Join([]string{name, repository, templateURL}, " "))
	integrationType := inferIntegrationType(signal)
	category := inferServiceCategory(integrationType)
	confidence := "generic-web"
	sources := []string{"container-name"}
	if integrationType != "" {
		confidence = "known-service"
		sources = append(sources, "service-signature")
	}
	return DashboardService{
		DisplayName:     name,
		Slug:            slug,
		IntegrationType: integrationType,
		Category:        category,
		Icon:            InferIconName(dockerxml.Template{Name: name}),
		Confidence:      confidence,
		Sources:         sources,
	}
}

func dockerLabelState(state map[string]string) map[string]string {
	labels := map[string]string{}
	for key, value := range state {
		if strings.HasPrefix(key, "amud.") || strings.Contains(key, ".") {
			labels[key] = value
		}
	}
	return labels
}

func inferIntegrationType(signal string) string {
	normalized := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(signal), "-")
	for _, item := range knownDashboardIntegrations {
		if strings.Contains(normalized, item.token) {
			return item.id
		}
	}
	return ""
}

func inferServiceCategory(integrationType string) string {
	switch integrationType {
	case "radarr", "sonarr", "lidarr", "readarr", "whisparr", "prowlarr", "bazarr", "overseerr", "jellyseerr":
		return "Servarr & requests"
	case "plex", "jellyfin", "emby", "tautulli", "immich", "photoprism", "navidrome", "komga":
		return "Media & photos"
	case "qbittorrent", "sabnzbd", "nzbget", "transmission", "deluge", "jackett":
		return "Download"
	case "pihole", "adguard", "technitium", "blocky":
		return "DNS & adblock"
	case "uptime_kuma", "grafana", "netdata", "glances", "beszel", "prometheus":
		return "Monitoring"
	case "unraid", "proxmox", "portainer", "nginx_proxy_manager", "traefik", "cloudflare_tunnel":
		return "Network & infra"
	default:
		return ""
	}
}

type integrationSignature struct {
	token string
	id    string
}

var knownDashboardIntegrations = []integrationSignature{
	{token: "cloudflare-tunnel", id: "cloudflare_tunnel"},
	{token: "nginx-proxy-manager", id: "nginx_proxy_manager"},
	{token: "uptime-kuma", id: "uptime_kuma"},
	{token: "jellyseerr", id: "jellyseerr"},
	{token: "overseerr", id: "overseerr"},
	{token: "prowlarr", id: "prowlarr"},
	{token: "bazarr", id: "bazarr"},
	{token: "radarr", id: "radarr"},
	{token: "sonarr", id: "sonarr"},
	{token: "lidarr", id: "lidarr"},
	{token: "readarr", id: "readarr"},
	{token: "whisparr", id: "whisparr"},
	{token: "qbittorrent", id: "qbittorrent"},
	{token: "sabnzbd", id: "sabnzbd"},
	{token: "nzbget", id: "nzbget"},
	{token: "transmission", id: "transmission"},
	{token: "deluge", id: "deluge"},
	{token: "jackett", id: "jackett"},
	{token: "tautulli", id: "tautulli"},
	{token: "plex", id: "plex"},
	{token: "jellyfin", id: "jellyfin"},
	{token: "emby", id: "emby"},
	{token: "immich", id: "immich"},
	{token: "photoprism", id: "photoprism"},
	{token: "navidrome", id: "navidrome"},
	{token: "komga", id: "komga"},
	{token: "pihole", id: "pihole"},
	{token: "adguard", id: "adguard"},
	{token: "technitium", id: "technitium"},
	{token: "blocky", id: "blocky"},
	{token: "grafana", id: "grafana"},
	{token: "netdata", id: "netdata"},
	{token: "glances", id: "glances"},
	{token: "beszel", id: "beszel"},
	{token: "prometheus", id: "prometheus"},
	{token: "unraid", id: "unraid"},
	{token: "proxmox", id: "proxmox"},
	{token: "portainer", id: "portainer"},
	{token: "traefik", id: "traefik"},
}

func hashDashboardPlan(plan DashboardPlan) string {
	plan.PlanHash = ""
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
