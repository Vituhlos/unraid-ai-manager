package xmlpatch

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"unraid-ai-manager/internal/dockerxml"
)

var managedAMUDLabelOrder = []string{
	"amud.enable",
	"amud.url",
	"amud.name",
	"amud.icon",
}

var managedAMUDLabelNames = map[string]string{
	"amud.enable": "AMUD Enable",
	"amud.url":    "AMUD URL",
	"amud.name":   "AMUD Name",
	"amud.icon":   "AMUD Icon",
}

type Operation struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Line   int    `json:"line,omitempty"`
}

type Result struct {
	Original string      `json:"original"`
	Modified string      `json:"modified"`
	Changed  bool        `json:"changed"`
	Ops      []Operation `json:"ops"`
}

func ApplyAMUDLabels(original string, labels map[string]string) (Result, error) {
	if _, err := parseTemplateString(original); err != nil {
		return Result{}, fmt.Errorf("original XML is invalid: %w", err)
	}

	lineEnding := detectLineEnding(original)
	normalized := strings.ReplaceAll(original, "\r\n", "\n")
	hadFinalNewline := strings.HasSuffix(normalized, "\n")
	lines := splitLines(normalized)

	var ops []Operation
	keys := orderedManagedKeys(labels)
	for _, key := range keys {
		value := labels[key]
		index := findConfigLine(lines, key)
		if index >= 0 {
			indent := leadingWhitespace(lines[index])
			replacement := amudLabelLine(indent, key, value)
			if lines[index] != replacement {
				lines[index] = replacement
				ops = append(ops, Operation{Action: "update", Key: key, Line: index + 1})
			} else {
				ops = append(ops, Operation{Action: "unchanged", Key: key, Line: index + 1})
			}
			continue
		}

		insertAt, indent, err := findInsertionPoint(lines)
		if err != nil {
			return Result{}, err
		}
		line := amudLabelLine(indent, key, value)
		lines = insertLine(lines, insertAt, line)
		ops = append(ops, Operation{Action: "add", Key: key, Line: insertAt + 1})
	}

	modified := strings.Join(lines, "\n")
	if hadFinalNewline {
		modified += "\n"
	}
	if lineEnding == "\r\n" {
		modified = strings.ReplaceAll(modified, "\n", "\r\n")
	}

	if _, err := parseTemplateString(modified); err != nil {
		return Result{}, fmt.Errorf("modified XML is invalid: %w", err)
	}

	return Result{
		Original: original,
		Modified: modified,
		Changed:  original != modified,
		Ops:      ops,
	}, nil
}

func orderedManagedKeys(labels map[string]string) []string {
	var keys []string
	for _, key := range managedAMUDLabelOrder {
		if _, ok := labels[key]; ok {
			keys = append(keys, key)
		}
	}
	extras := make([]string, 0)
	for key := range labels {
		if strings.HasPrefix(key, "amud.") && !contains(keys, key) {
			extras = append(extras, key)
		}
	}
	sort.Strings(extras)
	return append(keys, extras...)
}

func findConfigLine(lines []string, target string) int {
	targetPattern := `Target="` + regexp.QuoteMeta(target) + `"`
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "<Config ") {
			continue
		}
		if strings.Contains(trimmed, `Type="Label"`) && regexp.MustCompile(targetPattern).MatchString(trimmed) {
			return index
		}
	}
	return -1
}

func findInsertionPoint(lines []string) (int, string, error) {
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "<TailscaleStateDir") {
			return index, leadingWhitespace(line), nil
		}
	}
	for index, line := range lines {
		if strings.TrimSpace(line) == "</Container>" {
			indent := leadingWhitespace(line)
			if indent == "" {
				indent = "  "
			}
			return index, indent, nil
		}
	}
	return 0, "", fmt.Errorf("could not find a safe insertion point before TailscaleStateDir or </Container>")
}

func amudLabelLine(indent string, key string, value string) string {
	name := managedAMUDLabelNames[key]
	if name == "" {
		name = "AMUD " + key
	}
	escapedValue := escapeXML(value)
	return fmt.Sprintf(
		`%s<Config Name="%s" Target="%s" Default="%s" Mode="" Description="Managed by Unraid AI Manager" Type="Label" Display="advanced" Required="false" Mask="false">%s</Config>`,
		indent,
		escapeXML(name),
		escapeXML(key),
		escapedValue,
		escapedValue,
	)
}

func parseTemplateString(value string) (dockerxml.Template, error) {
	var template dockerxml.Template
	if err := xml.Unmarshal([]byte(value), &template); err != nil {
		return dockerxml.Template{}, err
	}
	if template.XMLName.Local != "Container" {
		return dockerxml.Template{}, fmt.Errorf("root element is %q, expected Container", template.XMLName.Local)
	}
	return template, nil
}

func detectLineEnding(value string) string {
	if strings.Contains(value, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func splitLines(value string) []string {
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "\n")
}

func leadingWhitespace(value string) string {
	return value[:len(value)-len(strings.TrimLeft(value, " \t"))]
}

func insertLine(lines []string, index int, line string) []string {
	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = line
	return lines
}

func escapeXML(value string) string {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		return value
	}
	return buffer.String()
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
