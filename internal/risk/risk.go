package risk

import (
	"strings"

	"unraid-ai-manager/internal/dockerxml"
)

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
}

func AnalyzeTemplate(template dockerxml.Template) []Finding {
	var findings []Finding

	if template.Privileged {
		findings = append(findings, Finding{
			Severity: "high",
			Code:     "privileged_container",
			Message:  "Template enables Privileged=true.",
			Field:    "Privileged",
		})
	}

	if strings.EqualFold(template.Network, "host") {
		findings = append(findings, Finding{
			Severity: "review",
			Code:     "host_network",
			Message:  "Container uses host networking; port inference and exposure need review.",
			Field:    "Network",
		})
	}

	findings = append(findings, analyzeExtraParams(template.ExtraParams)...)
	findings = append(findings, analyzePaths(template)...)

	if template.TemplateURL == "" {
		findings = append(findings, Finding{
			Severity: "info",
			Code:     "missing_template_url",
			Message:  "TemplateURL is empty; this may be a custom/non-CA template.",
			Field:    "TemplateURL",
		})
	}

	return findings
}

func analyzeExtraParams(extraParams string) []Finding {
	if strings.TrimSpace(extraParams) == "" {
		return nil
	}

	tokens, ok := splitCommandLine(extraParams)
	if !ok {
		return []Finding{{
			Severity: "review",
			Code:     "extra_params_parse_failed",
			Message:  "ExtraParams could not be parsed safely: " + extraParams,
			Field:    "ExtraParams",
		}}
	}

	var findings []Finding
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token == "--init" || strings.HasPrefix(token, "--user=") || strings.HasPrefix(token, "--restart=") {
			continue
		}
		if token == "--user" {
			index++
			continue
		}
		if hasAnyPrefix(token, []string{
			"--privileged",
			"--cap-add",
			"--device",
			"--mount",
			"--volume",
			"-v",
			"--pid=host",
			"--ipc=host",
			"--uts=host",
			"--network=host",
		}) {
			findings = append(findings, Finding{
				Severity: "high",
				Code:     "dangerous_extra_param",
				Message:  "ExtraParams contains potentially dangerous option: " + token,
				Field:    "ExtraParams",
			})
			continue
		}
		findings = append(findings, Finding{
			Severity: "review",
			Code:     "unclassified_extra_param",
			Message:  "ExtraParams contains option that should be reviewed: " + token,
			Field:    "ExtraParams",
		})
	}
	return findings
}

func analyzePaths(template dockerxml.Template) []Finding {
	var findings []Finding
	for _, pathConfig := range template.Paths() {
		hostPath := normalizeHostPath(pathConfig.Value)
		if hostPath == "" {
			continue
		}

		if isBroadMount(hostPath) {
			findings = append(findings, Finding{
				Severity: "high",
				Code:     "broad_host_mount",
				Message:  "Path maps a broad host location: " + hostPath,
				Field:    pathConfig.Name,
			})
			continue
		}

		if hostPath == "/var/run/docker.sock" {
			findings = append(findings, Finding{
				Severity: "high",
				Code:     "docker_socket_mount",
				Message:  "Path maps /var/run/docker.sock.",
				Field:    pathConfig.Name,
			})
			continue
		}

		if hasAnyPrefix(hostPath, []string{"/boot/", "/etc/", "/usr/", "/var/run/"}) {
			findings = append(findings, Finding{
				Severity: "high",
				Code:     "sensitive_host_mount",
				Message:  "Path maps sensitive host location: " + hostPath,
				Field:    pathConfig.Name,
			})
			continue
		}

		if strings.HasPrefix(hostPath, "/mnt/user/") && !strings.HasPrefix(hostPath, "/mnt/user/appdata/") {
			severity := "info"
			mode := strings.ToLower(pathConfig.Mode)
			if mode == "rw" || mode == "rw,slave" || mode == "rw,shared" {
				severity = "review"
			}
			findings = append(findings, Finding{
				Severity: severity,
				Code:     "user_share_mount",
				Message:  "Path maps a user share location: " + hostPath,
				Field:    pathConfig.Name,
			})
		}
	}
	return findings
}

func splitCommandLine(value string) ([]string, bool) {
	var tokens []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range value {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}

	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, false
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, true
}

func normalizeHostPath(path string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	for strings.Contains(normalized, "//") {
		normalized = strings.ReplaceAll(normalized, "//", "/")
	}
	if normalized != "/" {
		normalized = strings.TrimRight(normalized, "/")
	}
	return normalized
}

func isBroadMount(path string) bool {
	switch path {
	case "/", "/boot", "/etc", "/usr", "/var", "/mnt", "/mnt/user", "/mnt/cache":
		return true
	default:
		return false
	}
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
