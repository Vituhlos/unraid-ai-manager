package xmlpatch

import (
	"fmt"
	"regexp"
	"strings"
)

func ApplyVariable(original string, target string, value string, displayName string) (Result, error) {
	if _, err := parseTemplateString(original); err != nil {
		return Result{}, fmt.Errorf("original XML is invalid: %w", err)
	}

	lineEnding := detectLineEnding(original)
	normalized := strings.ReplaceAll(original, "\r\n", "\n")
	hadFinalNewline := strings.HasSuffix(normalized, "\n")
	lines := splitLines(normalized)

	ops := []Operation{}
	index := findConfigLineByTypeAndTarget(lines, "Variable", target)
	if index >= 0 {
		indent := leadingWhitespace(lines[index])
		replacement := variableLine(indent, target, value, displayName)
		if lines[index] != replacement {
			lines[index] = replacement
			ops = append(ops, Operation{Action: "update", Key: target, Line: index + 1})
		} else {
			ops = append(ops, Operation{Action: "unchanged", Key: target, Line: index + 1})
		}
	} else {
		insertAt, indent, err := findInsertionPoint(lines)
		if err != nil {
			return Result{}, err
		}
		lines = insertLine(lines, insertAt, variableLine(indent, target, value, displayName))
		ops = append(ops, Operation{Action: "add", Key: target, Line: insertAt + 1})
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

func findConfigLineByTypeAndTarget(lines []string, configType string, target string) int {
	targetPattern := `Target="` + regexp.QuoteMeta(target) + `"`
	typePattern := `Type="` + regexp.QuoteMeta(configType) + `"`
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "<Config ") {
			continue
		}
		if regexp.MustCompile(typePattern).MatchString(trimmed) && regexp.MustCompile(targetPattern).MatchString(trimmed) {
			return index
		}
	}
	return -1
}

func variableLine(indent string, target string, value string, displayName string) string {
	if displayName == "" {
		displayName = target
	}
	escapedValue := escapeXML(value)
	return fmt.Sprintf(
		`%s<Config Name="%s" Target="%s" Default="%s" Mode="" Description="Managed by Unraid AI Manager" Type="Variable" Display="advanced" Required="false" Mask="false">%s</Config>`,
		indent,
		escapeXML(displayName),
		escapeXML(target),
		escapedValue,
		escapedValue,
	)
}
