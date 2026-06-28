package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"unraid-ai-manager/internal/dockerxml"
)

type TZPlan struct {
	Kind         string    `json:"kind"`
	WriteEnabled bool      `json:"write_enabled"`
	Timezone     string    `json:"timezone"`
	Entries      []TZEntry `json:"entries"`
	PlanHash     string    `json:"plan_hash"`
}

type TZEntry struct {
	Container     string  `json:"container"`
	SourcePath    string  `json:"source_path"`
	Action        string  `json:"action"`
	CurrentValue  *string `json:"current_value,omitempty"`
	ProposedValue string  `json:"proposed_value"`
}

type TZOptions struct {
	Timezone         string
	Names            map[string]bool
	IncludeUnchanged bool
}

func BuildTZPlan(templates []dockerxml.Template, options TZOptions) TZPlan {
	plan := TZPlan{
		Kind:         "template-tz",
		WriteEnabled: false,
		Timezone:     options.Timezone,
	}

	for _, template := range templates {
		if len(options.Names) > 0 && !options.Names[strings.ToLower(template.Name)] {
			continue
		}
		current, exists := currentVariableValue(template, "TZ")
		action := "add"
		var currentPointer *string
		if exists {
			currentPointer = &current
			if current == options.Timezone {
				action = "unchanged"
			} else {
				action = "update"
			}
		}
		if action == "unchanged" && !options.IncludeUnchanged {
			continue
		}
		plan.Entries = append(plan.Entries, TZEntry{
			Container:     template.Name,
			SourcePath:    template.SourcePath,
			Action:        action,
			CurrentValue:  currentPointer,
			ProposedValue: options.Timezone,
		})
	}

	plan.PlanHash = hashTZPlan(plan)
	return plan
}

func currentVariableValue(template dockerxml.Template, target string) (string, bool) {
	for _, variable := range template.Variables() {
		if variable.Target == target {
			return variable.Value, true
		}
	}
	return "", false
}

func hashTZPlan(plan TZPlan) string {
	plan.PlanHash = ""
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
