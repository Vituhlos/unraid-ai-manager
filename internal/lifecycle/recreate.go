package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"unraid-ai-manager/internal/compare"
	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/risk"
)

type RecreatePlan struct {
	Kind         string          `json:"kind"`
	WriteEnabled bool            `json:"write_enabled"`
	Entries      []RecreateEntry `json:"entries"`
	PlanHash     string          `json:"plan_hash"`
}

type RecreateEntry struct {
	Container       string            `json:"container"`
	TemplatePath    string            `json:"template_path"`
	RuntimeSource   string            `json:"runtime_source"`
	State           string            `json:"state"`
	Reasons         []string          `json:"reasons"`
	Preflight       []PreflightCheck  `json:"preflight"`
	RiskFindings    []risk.Finding    `json:"risk_findings,omitempty"`
	RequiresConfirm bool              `json:"requires_confirm"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type PreflightCheck struct {
	Code     string `json:"code"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

type Options struct {
	IncludeAll bool
	Names      map[string]bool
}

func BuildRecreatePlan(templates []dockerxml.Template, runtime []dockerinspect.Container, options Options) RecreatePlan {
	report := compare.RuntimeVsTemplates(templates, runtime)
	templateByName := map[string]dockerxml.Template{}
	for _, template := range templates {
		templateByName[strings.ToLower(template.Name)] = template
	}

	plan := RecreatePlan{
		Kind:         "docker-recreate",
		WriteEnabled: false,
	}

	for _, match := range report.Matches {
		if len(options.Names) > 0 && !options.Names[strings.ToLower(match.Name)] {
			continue
		}
		template := templateByName[strings.ToLower(match.Name)]
		reasons := reasonsFor(match)
		if len(reasons) == 0 && !options.IncludeAll {
			continue
		}

		findings := risk.AnalyzeTemplate(template)
		entry := RecreateEntry{
			Container:       match.Name,
			TemplatePath:    match.TemplatePath,
			RuntimeSource:   match.RuntimeSource,
			State:           runtimeState(match.Name, runtime),
			Reasons:         reasons,
			Preflight:       preflightChecks(match, findings),
			RiskFindings:    findings,
			RequiresConfirm: true,
			Metadata: map[string]string{
				"repository":       match.Repository,
				"runtime_image":    match.RuntimeImage,
				"template_network": match.TemplateNetwork,
				"runtime_network":  match.RuntimeNetwork,
			},
		}
		plan.Entries = append(plan.Entries, entry)
	}

	plan.PlanHash = hashPlan(plan)
	return plan
}

func reasonsFor(match compare.ContainerReport) []string {
	var reasons []string
	for _, label := range match.LabelComparisons {
		if !label.Match {
			reasons = append(reasons, "runtime AMUD label "+label.Key+" differs from template")
		}
	}
	for _, env := range match.EnvComparisons {
		if !env.Match {
			reasons = append(reasons, "runtime env "+env.Key+" differs from template")
		}
	}
	for _, port := range match.PortComparisons {
		if !port.Match {
			reasons = append(reasons, "runtime port "+port.ContainerPort+"/"+port.Protocol+" differs from template")
		}
	}
	for _, warning := range match.Warnings {
		if strings.Contains(warning, "repository") {
			reasons = append(reasons, "runtime image differs from template repository")
		}
		if strings.Contains(warning, "network") {
			reasons = append(reasons, "runtime network differs from template network")
		}
	}
	return reasons
}

func preflightChecks(match compare.ContainerReport, findings []risk.Finding) []PreflightCheck {
	checks := []PreflightCheck{
		{
			Code:    "runtime_container_matched",
			OK:      true,
			Message: "Runtime container matches DockerMan template by name.",
		},
	}

	networkOK := match.TemplateNetwork == "" || match.RuntimeNetwork == "" || strings.EqualFold(match.TemplateNetwork, match.RuntimeNetwork)
	checks = append(checks, PreflightCheck{
		Code:    "network_matches",
		OK:      networkOK,
		Message: boolMessage(networkOK, "Template and runtime network match.", "Template and runtime network differ."),
	})

	highRisk := false
	for _, finding := range findings {
		if finding.Severity == "high" {
			highRisk = true
			break
		}
	}
	checks = append(checks, PreflightCheck{
		Code:     "no_high_risk_template_findings",
		OK:       !highRisk,
		Message:  boolMessage(!highRisk, "No high-risk template findings.", "Template has high-risk findings."),
		Severity: boolSeverity(!highRisk),
	})

	return checks
}

func runtimeState(name string, runtime []dockerinspect.Container) string {
	for _, container := range runtime {
		if strings.EqualFold(container.Name, name) {
			return container.State
		}
	}
	return ""
}

func boolMessage(ok bool, ifOK string, ifNotOK string) string {
	if ok {
		return ifOK
	}
	return ifNotOK
}

func boolSeverity(ok bool) string {
	if ok {
		return ""
	}
	return "high"
}

func hashPlan(plan RecreatePlan) string {
	plan.PlanHash = ""
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
