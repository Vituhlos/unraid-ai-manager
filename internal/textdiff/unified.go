package textdiff

import (
	"fmt"
	"strings"
)

type opKind int

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

type op struct {
	kind    opKind
	text    string
	oldLine int
	newLine int
}

func Unified(oldName string, newName string, oldText string, newText string, context int) string {
	if oldText == newText {
		return ""
	}
	if context < 0 {
		context = 3
	}

	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	ops := diffOps(oldLines, newLines)

	first, last := changedRange(ops)
	if first == -1 {
		return ""
	}
	start := first - context
	if start < 0 {
		start = 0
	}
	end := last + context + 1
	if end > len(ops) {
		end = len(ops)
	}
	hunkOps := ops[start:end]

	oldStart, oldCount, newStart, newCount := hunkRange(hunkOps)

	var builder strings.Builder
	builder.WriteString("--- " + oldName + "\n")
	builder.WriteString("+++ " + newName + "\n")
	builder.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))
	for _, op := range hunkOps {
		switch op.kind {
		case opEqual:
			builder.WriteString(" " + op.text + "\n")
		case opDelete:
			builder.WriteString("-" + op.text + "\n")
		case opInsert:
			builder.WriteString("+" + op.text + "\n")
		}
	}
	return builder.String()
}

func diffOps(oldLines []string, newLines []string) []op {
	n := len(oldLines)
	m := len(newLines)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		if oldLines[i] == newLines[j] {
			ops = append(ops, op{kind: opEqual, text: oldLines[i], oldLine: i + 1, newLine: j + 1})
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, op{kind: opDelete, text: oldLines[i], oldLine: i + 1})
			i++
		} else {
			ops = append(ops, op{kind: opInsert, text: newLines[j], newLine: j + 1})
			j++
		}
	}
	for i < n {
		ops = append(ops, op{kind: opDelete, text: oldLines[i], oldLine: i + 1})
		i++
	}
	for j < m {
		ops = append(ops, op{kind: opInsert, text: newLines[j], newLine: j + 1})
		j++
	}
	return ops
}

func changedRange(ops []op) (int, int) {
	first := -1
	last := -1
	for index, op := range ops {
		if op.kind == opEqual {
			continue
		}
		if first == -1 {
			first = index
		}
		last = index
	}
	return first, last
}

func hunkRange(ops []op) (int, int, int, int) {
	oldStart := 0
	newStart := 0
	oldCount := 0
	newCount := 0
	for _, op := range ops {
		if oldStart == 0 && op.oldLine > 0 {
			oldStart = op.oldLine
		}
		if newStart == 0 && op.newLine > 0 {
			newStart = op.newLine
		}
		if op.kind != opInsert {
			oldCount++
		}
		if op.kind != opDelete {
			newCount++
		}
	}
	if oldStart == 0 {
		oldStart = 1
	}
	if newStart == 0 {
		newStart = 1
	}
	return oldStart, oldCount, newStart, newCount
}

func splitLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "\n")
}
