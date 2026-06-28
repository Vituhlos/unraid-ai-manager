package compare

import (
	"sort"
	"strings"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
)

type Report struct {
	TemplateCount    int               `json:"template_count"`
	RuntimeCount     int               `json:"runtime_count"`
	Matches          []ContainerReport `json:"matches"`
	MissingRuntime   []string          `json:"missing_runtime"`
	UnmatchedRuntime []string          `json:"unmatched_runtime"`
}

type ContainerReport struct {
	Name             string            `json:"name"`
	TemplatePath     string            `json:"template_path"`
	RuntimeSource    string            `json:"runtime_source"`
	Repository       string            `json:"repository"`
	RuntimeImage     string            `json:"runtime_image"`
	TemplateNetwork  string            `json:"template_network"`
	RuntimeNetwork   string            `json:"runtime_network"`
	PortComparisons  []PortComparison  `json:"port_comparisons"`
	LabelComparisons []LabelComparison `json:"label_comparisons"`
	EnvComparisons   []EnvComparison   `json:"env_comparisons"`
	Warnings         []string          `json:"warnings"`
}

type PortComparison struct {
	ContainerPort    string `json:"container_port"`
	Protocol         string `json:"protocol"`
	TemplateHostPort string `json:"template_host_port"`
	RuntimeHostPort  string `json:"runtime_host_port"`
	Match            bool   `json:"match"`
}

type LabelComparison struct {
	Key           string `json:"key"`
	TemplateValue string `json:"template_value,omitempty"`
	RuntimeValue  string `json:"runtime_value,omitempty"`
	Match         bool   `json:"match"`
}

type EnvComparison struct {
	Key           string `json:"key"`
	TemplateValue string `json:"template_value,omitempty"`
	RuntimeValue  string `json:"runtime_value,omitempty"`
	Match         bool   `json:"match"`
}

func RuntimeVsTemplates(templates []dockerxml.Template, runtime []dockerinspect.Container) Report {
	runtimeIndex := dockerinspect.IndexByName(runtime)
	usedRuntime := map[string]bool{}

	report := Report{
		TemplateCount: len(templates),
		RuntimeCount:  len(runtime),
	}

	for _, template := range templates {
		container, ok := lookupRuntime(runtimeIndex, template.Name)
		if !ok {
			report.MissingRuntime = append(report.MissingRuntime, template.Name)
			continue
		}
		usedRuntime[strings.ToLower(container.Name)] = true
		report.Matches = append(report.Matches, compareOne(template, container))
	}

	for _, container := range runtime {
		if !usedRuntime[strings.ToLower(container.Name)] {
			report.UnmatchedRuntime = append(report.UnmatchedRuntime, container.Name)
		}
	}
	sort.Strings(report.MissingRuntime)
	sort.Strings(report.UnmatchedRuntime)
	return report
}

func compareOne(template dockerxml.Template, runtime dockerinspect.Container) ContainerReport {
	report := ContainerReport{
		Name:            template.Name,
		TemplatePath:    template.SourcePath,
		RuntimeSource:   runtime.RawSourcePath,
		Repository:      template.Repository,
		RuntimeImage:    runtime.Image,
		TemplateNetwork: template.Network,
		RuntimeNetwork:  runtime.NetworkMode,
	}

	if template.Repository != "" && runtime.Image != "" && template.Repository != runtime.Image {
		report.Warnings = append(report.Warnings, "Template repository differs from runtime image.")
	}
	if template.Network != "" && runtime.NetworkMode != "" && !strings.EqualFold(template.Network, runtime.NetworkMode) {
		report.Warnings = append(report.Warnings, "Template network differs from runtime network.")
	}

	report.PortComparisons = comparePorts(template.Ports(), runtime.Ports)
	report.LabelComparisons = compareLabels(template.LabelMap(), runtime.Labels)
	report.EnvComparisons = compareEnv(template.Variables(), runtime.Env)
	return report
}

func comparePorts(templatePorts []dockerxml.ConfigEntry, runtimePorts []dockerinspect.Port) []PortComparison {
	runtimeIndex := map[string]dockerinspect.Port{}
	for _, port := range runtimePorts {
		key := portKey(port.ContainerPort, port.Protocol)
		if _, exists := runtimeIndex[key]; !exists {
			runtimeIndex[key] = port
		}
	}

	var comparisons []PortComparison
	for _, templatePort := range templatePorts {
		key := portKey(templatePort.Target, templatePort.Mode)
		runtimePort := runtimeIndex[key]
		comparisons = append(comparisons, PortComparison{
			ContainerPort:    templatePort.Target,
			Protocol:         strings.ToLower(templatePort.Mode),
			TemplateHostPort: templatePort.Value,
			RuntimeHostPort:  runtimePort.HostPort,
			Match:            templatePort.Value == runtimePort.HostPort,
		})
	}
	return comparisons
}

func compareLabels(templateLabels map[string]string, runtimeLabels map[string]string) []LabelComparison {
	keys := interestingLabelKeys(templateLabels, runtimeLabels)
	comparisons := make([]LabelComparison, 0, len(keys))
	for _, key := range keys {
		templateValue := templateLabels[key]
		runtimeValue := runtimeLabels[key]
		comparisons = append(comparisons, LabelComparison{
			Key:           key,
			TemplateValue: templateValue,
			RuntimeValue:  runtimeValue,
			Match:         templateValue == runtimeValue,
		})
	}
	return comparisons
}

func compareEnv(templateVariables []dockerxml.ConfigEntry, runtimeEnv map[string]string) []EnvComparison {
	var comparisons []EnvComparison
	for _, variable := range templateVariables {
		runtimeValue, ok := runtimeEnv[variable.Target]
		if !ok {
			continue
		}
		comparisons = append(comparisons, EnvComparison{
			Key:           variable.Target,
			TemplateValue: variable.Value,
			RuntimeValue:  runtimeValue,
			Match:         variable.Value == runtimeValue,
		})
	}
	return comparisons
}

func interestingLabelKeys(templateLabels map[string]string, runtimeLabels map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	for key := range templateLabels {
		if strings.HasPrefix(key, "amud.") {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for key := range runtimeLabels {
		if strings.HasPrefix(key, "amud.") && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func lookupRuntime(index map[string]dockerinspect.Container, name string) (dockerinspect.Container, bool) {
	if container, ok := index[name]; ok {
		return container, true
	}
	container, ok := index[strings.ToLower(name)]
	return container, ok
}

func portKey(containerPort string, protocol string) string {
	return containerPort + "/" + strings.ToLower(protocol)
}
