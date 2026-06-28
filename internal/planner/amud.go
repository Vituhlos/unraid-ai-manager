package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/risk"
)

type AMUDOptions struct {
	LocalHost        string
	URLMode          string
	CloudflareDomain string
	CloudflareRoutes map[string]string
	Names            map[string]bool
	ExcludedNames    map[string]bool
	IncludePortOnly  bool
	RuntimeFilter    string
}

type AMUDPlan struct {
	Kind             string      `json:"kind"`
	WriteEnabled     bool        `json:"write_enabled"`
	URLMode          string      `json:"url_mode"`
	LocalHost        string      `json:"local_host"`
	CloudflareDomain string      `json:"cloudflare_domain,omitempty"`
	IncludePortOnly  bool        `json:"include_port_only,omitempty"`
	RuntimeFilter    string      `json:"runtime_filter"`
	Entries          []AMUDEntry `json:"entries"`
	PlanHash         string      `json:"plan_hash"`
}

type AMUDEntry struct {
	Container      string            `json:"container"`
	SourcePath     string            `json:"source_path"`
	Repository     string            `json:"repository"`
	TemplateURL    string            `json:"template_url"`
	WebDetection   WebDetection      `json:"web_detection"`
	URL            URLResult         `json:"url"`
	CurrentLabels  map[string]string `json:"current_labels"`
	ProposedLabels map[string]string `json:"proposed_labels"`
	LabelChanges   []LabelChange     `json:"label_changes"`
	Warnings       []string          `json:"warnings"`
}

type WebDetection struct {
	Confidence    string `json:"confidence"`
	Reason        string `json:"reason"`
	WebUI         string `json:"web_ui"`
	ContainerPort *int   `json:"container_port"`
}

type URLResult struct {
	Mode          string   `json:"mode"`
	Source        string   `json:"source"`
	URL           string   `json:"url"`
	HostPort      string   `json:"host_port,omitempty"`
	ContainerPort string   `json:"container_port,omitempty"`
	Warnings      []string `json:"warnings"`
}

type LabelChange struct {
	Action   string  `json:"action"`
	Key      string  `json:"key"`
	Current  *string `json:"current"`
	Proposed string  `json:"proposed"`
}

func BuildAMUDPlan(templates []dockerxml.Template, options AMUDOptions) AMUDPlan {
	if options.URLMode == "" {
		options.URLMode = "local"
	}
	if options.CloudflareRoutes == nil {
		options.CloudflareRoutes = map[string]string{}
	}
	if options.RuntimeFilter == "" {
		options.RuntimeFilter = "templates"
	}

	plan := AMUDPlan{
		Kind:             "amud-labels",
		WriteEnabled:     false,
		URLMode:          options.URLMode,
		LocalHost:        options.LocalHost,
		CloudflareDomain: options.CloudflareDomain,
		IncludePortOnly:  options.IncludePortOnly,
		RuntimeFilter:    options.RuntimeFilter,
	}

	for _, template := range templates {
		if !nameIncluded(template.Name, options.Names) || nameMatches(template.Name, options.ExcludedNames) {
			continue
		}

		web := DetectWebCandidate(template)
		if web.Confidence == "none" {
			continue
		}
		if web.Confidence == "medium" && !options.IncludePortOnly && len(options.Names) == 0 {
			continue
		}

		url := ResolveAMUDURL(template, options)
		proposed := map[string]string{
			"amud.enable": "true",
			"amud.name":   template.Name,
			"amud.icon":   InferIconName(template),
		}
		if url.URL != "" {
			proposed["amud.url"] = url.URL
		}

		current := template.LabelMap()
		changes := buildLabelChanges(current, proposed)

		warnings := append([]string{}, url.Warnings...)
		for _, finding := range risk.AnalyzeTemplate(template) {
			if finding.Severity == "high" || finding.Severity == "review" {
				warnings = append(warnings, strings.ToUpper(finding.Severity)+": "+finding.Message)
			}
		}

		plan.Entries = append(plan.Entries, AMUDEntry{
			Container:      template.Name,
			SourcePath:     template.SourcePath,
			Repository:     template.Repository,
			TemplateURL:    template.TemplateURL,
			WebDetection:   web,
			URL:            url,
			CurrentLabels:  current,
			ProposedLabels: proposed,
			LabelChanges:   changes,
			Warnings:       warnings,
		})
	}

	plan.PlanHash = hashPlan(plan)
	return plan
}

func NormalizeRuntimeFilter(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = "templates"
	}
	switch value {
	case "templates", "existing", "running":
		return value, nil
	default:
		return "", errors.New("runtime_filter must be templates, existing, or running")
	}
}

func FilterTemplatesByRuntime(templates []dockerxml.Template, runtime []dockerinspect.Container, mode string) ([]dockerxml.Template, error) {
	mode, err := NormalizeRuntimeFilter(mode)
	if err != nil {
		return nil, err
	}
	if mode == "templates" {
		return templates, nil
	}

	runtimeIndex := dockerinspect.IndexByName(runtime)
	filtered := make([]dockerxml.Template, 0, len(templates))
	for _, template := range templates {
		container, ok := lookupRuntimeByTemplateName(runtimeIndex, template.Name)
		if !ok {
			continue
		}
		if mode == "running" && !strings.EqualFold(container.State, "running") {
			continue
		}
		filtered = append(filtered, template)
	}
	return filtered, nil
}

func DetectWebCandidate(template dockerxml.Template) WebDetection {
	if template.WebUI != "" {
		return WebDetection{
			Confidence:    "high",
			Reason:        "Template has WebUI.",
			WebUI:         template.WebUI,
			ContainerPort: ParseWebUIContainerPort(template.WebUI),
		}
	}

	for _, port := range template.Ports() {
		if strings.EqualFold(port.Mode, "tcp") {
			containerPort := safeInt(port.Target)
			return WebDetection{
				Confidence:    "medium",
				Reason:        "Template has published TCP port but no WebUI.",
				WebUI:         "",
				ContainerPort: containerPort,
			}
		}
	}

	return WebDetection{
		Confidence: "none",
		Reason:     "No WebUI and no TCP ports.",
		WebUI:      "",
	}
}

func ResolveAMUDURL(template dockerxml.Template, options AMUDOptions) URLResult {
	if options.URLMode == "cloudflare" || options.URLMode == "hybrid" {
		if route := lookupRoute(options.CloudflareRoutes, template.Name); route != "" {
			return URLResult{
				Mode:   "cloudflare",
				Source: "explicit-route",
				URL:    cloudflareURL(route, options.CloudflareDomain),
			}
		}
		if options.URLMode == "cloudflare" {
			return URLResult{
				Mode:     "cloudflare",
				Source:   "missing-route",
				URL:      "",
				Warnings: []string{"No Cloudflare route known for " + template.Name + "; amud.url will not be proposed."},
			}
		}
	}

	local := ResolveLocalURL(template, options.LocalHost)
	return URLResult{
		Mode:          "local",
		Source:        local.Source,
		URL:           local.URL,
		HostPort:      local.HostPort,
		ContainerPort: local.ContainerPort,
		Warnings:      local.Warnings,
	}
}

func ResolveLocalURL(template dockerxml.Template, localHost string) URLResult {
	var warnings []string
	containerPort := ParseWebUIContainerPort(template.WebUI)
	var selected *dockerxml.ConfigEntry

	if containerPort != nil {
		selected = findPortByContainerPort(template, *containerPort)
	}

	if selected == nil {
		ports := template.Ports()
		for index := range ports {
			if strings.EqualFold(ports[index].Mode, "tcp") {
				selected = &ports[index]
				break
			}
		}
		if selected == nil && len(ports) > 0 {
			selected = &ports[0]
		}
		if selected != nil {
			warnings = append(warnings, "WebUI port did not match a Port config; using first TCP/available port.")
		}
	}

	if selected == nil {
		if strings.EqualFold(template.Network, "host") && containerPort != nil {
			port := strconv.Itoa(*containerPort)
			return URLResult{
				Source:        "host-network-webui",
				URL:           "http://" + localHost + ":" + port,
				HostPort:      port,
				ContainerPort: port,
				Warnings:      []string{"Host network template; using WebUI container port as host port."},
			}
		}
		return URLResult{
			Source:   "missing-port",
			URL:      "",
			Warnings: []string{"No usable port found for local AMUD URL."},
		}
	}

	if selected.Value == "" {
		return URLResult{
			Source:        "empty-host-port",
			URL:           "",
			HostPort:      "",
			ContainerPort: selected.Target,
			Warnings:      append(warnings, "Port config "+selected.Name+" has empty host port."),
		}
	}

	source := "first-port"
	if containerPort != nil {
		source = "webui-port"
	}
	return URLResult{
		Source:        source,
		URL:           "http://" + localHost + ":" + selected.Value,
		HostPort:      selected.Value,
		ContainerPort: selected.Target,
		Warnings:      warnings,
	}
}

func ParseWebUIContainerPort(webUI string) *int {
	matches := regexp.MustCompile(`\[PORT:(\d+)\]`).FindStringSubmatch(webUI)
	if len(matches) != 2 {
		return nil
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil
	}
	return &value
}

func InferIconName(template dockerxml.Template) string {
	return InferRouteKey(template.Name)
}

func InferRouteKey(name string) string {
	value := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(value, "-")
}

func buildLabelChanges(current map[string]string, proposed map[string]string) []LabelChange {
	keys := []string{"amud.enable", "amud.url", "amud.name", "amud.icon"}
	changes := make([]LabelChange, 0, len(keys))
	for _, key := range keys {
		proposedValue, ok := proposed[key]
		if !ok {
			continue
		}
		currentValue, exists := current[key]
		var currentPointer *string
		if exists {
			currentPointer = &currentValue
		}

		action := "add"
		if exists && currentValue == proposedValue {
			action = "unchanged"
		} else if exists {
			action = "update"
		}
		changes = append(changes, LabelChange{
			Action:   action,
			Key:      key,
			Current:  currentPointer,
			Proposed: proposedValue,
		})
	}
	return changes
}

func findPortByContainerPort(template dockerxml.Template, containerPort int) *dockerxml.ConfigEntry {
	target := strconv.Itoa(containerPort)
	ports := template.Ports()
	for index := range ports {
		if ports[index].Target == target && strings.EqualFold(ports[index].Mode, "tcp") {
			return &ports[index]
		}
	}
	for index := range ports {
		if ports[index].Target == target {
			return &ports[index]
		}
	}
	return nil
}

func lookupRoute(routes map[string]string, name string) string {
	for _, key := range []string{name, strings.ToLower(name), InferRouteKey(name)} {
		if route := routes[key]; route != "" {
			return route
		}
	}
	return ""
}

func nameIncluded(name string, names map[string]bool) bool {
	if len(names) == 0 {
		return true
	}
	return nameMatches(name, names)
}

func nameMatches(name string, names map[string]bool) bool {
	if len(names) == 0 {
		return false
	}
	for _, key := range []string{name, strings.ToLower(name), InferRouteKey(name)} {
		if names[key] {
			return true
		}
	}
	return false
}

func lookupRuntimeByTemplateName(index map[string]dockerinspect.Container, name string) (dockerinspect.Container, bool) {
	for _, key := range []string{name, strings.ToLower(name), InferRouteKey(name)} {
		if container, ok := index[key]; ok {
			return container, true
		}
	}
	return dockerinspect.Container{}, false
}

func cloudflareURL(route string, domain string) string {
	if strings.HasPrefix(route, "http://") || strings.HasPrefix(route, "https://") {
		return route
	}
	if domain == "" {
		return "https://" + route
	}
	return "https://" + route + "." + domain
}

func safeInt(value string) *int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func hashPlan(plan AMUDPlan) string {
	plan.PlanHash = ""
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
