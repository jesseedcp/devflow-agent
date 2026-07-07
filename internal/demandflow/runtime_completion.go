package demandflow

import (
	"sort"
	"strings"

	evidenceadapter "github.com/jesseedcp/devflow-agent/internal/evidence"
)

func (t RuntimeToolTrace) RedactedOutputExcerpt(maxBytes int) string {
	return evidenceadapter.Excerpt(t.Output, maxBytes)
}

func summarizeRuntimeTraces(stage Stage, model string, mode RuntimeCompletionMode, maxIterationsHit bool, traces []RuntimeToolTrace, changedFiles []string) RuntimeSummary {
	summary := RuntimeSummary{
		Stage:            stage,
		Model:            model,
		CompletionMode:   mode,
		MaxIterationsHit: maxIterationsHit,
		ToolCalls:        len(traces),
		ChangedFiles:     uniqueStrings(changedFiles),
	}
	for _, trace := range traces {
		if trace.IsError {
			summary.ErrorCalls++
		}
		switch trace.ToolName {
		case "WriteFile", "EditFile":
			summary.EditCalls++
		case "Bash":
			summary.BashCalls++
			if strings.TrimSpace(trace.Desc) != "" {
				summary.TestCommands = append(summary.TestCommands, strings.TrimSpace(trace.Desc))
			}
		}
		summary.LastTools = append(summary.LastTools, trace.ToolName)
	}
	summary.LastTools = lastRuntimeTools(summary.LastTools, 5)
	summary.TestCommands = uniqueStrings(summary.TestCommands)
	return summary
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
