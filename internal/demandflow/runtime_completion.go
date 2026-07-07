package demandflow

import (
	"os/exec"
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

func changedFilesFromRuntimeTraces(root string, traces []RuntimeToolTrace) []string {
	var files []string
	for _, trace := range traces {
		switch trace.ToolName {
		case "WriteFile", "EditFile":
			files = append(files, trace.Desc)
		}
	}
	files = append(files, changedFilesFromGit(root)...)
	return uniqueStrings(files)
}

func changedFilesFromGit(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	cmd := exec.Command("git", "-C", root, "diff", "--name-only", "HEAD", "--")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			files = append(files, trimmed)
		}
	}
	return uniqueStrings(files)
}

type runtimeImplementationEvidence struct {
	HasMutation            bool
	HasPassingTestCommand  bool
	ChangedFiles           []string
	TestCommands           []string
	PassingCommandExcerpts []string
}

func implementationEvidenceFromTraces(root string, traces []RuntimeToolTrace) runtimeImplementationEvidence {
	evidence := runtimeImplementationEvidence{}
	for _, trace := range traces {
		switch trace.ToolName {
		case "WriteFile", "EditFile":
			if strings.TrimSpace(trace.Desc) != "" && !trace.IsError {
				evidence.HasMutation = true
				evidence.ChangedFiles = append(evidence.ChangedFiles, trace.Desc)
			}
		case "Bash":
			command := strings.TrimSpace(trace.Desc)
			if command == "" {
				continue
			}
			if looksLikeTestCommand(command) {
				evidence.TestCommands = append(evidence.TestCommands, command)
				if !trace.IsError {
					evidence.HasPassingTestCommand = true
					evidence.PassingCommandExcerpts = append(evidence.PassingCommandExcerpts, trace.RedactedOutputExcerpt(512))
				}
			}
		}
	}
	evidence.ChangedFiles = uniqueStrings(append(evidence.ChangedFiles, changedFilesFromRuntimeTraces(root, traces)...))
	evidence.TestCommands = uniqueStrings(evidence.TestCommands)
	evidence.PassingCommandExcerpts = uniqueStrings(evidence.PassingCommandExcerpts)
	return evidence
}

func looksLikeTestCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(lower, "go test") ||
		strings.Contains(lower, "npm test") ||
		strings.Contains(lower, "pnpm test") ||
		strings.Contains(lower, "yarn test") ||
		strings.Contains(lower, "pytest") ||
		strings.Contains(lower, "cargo test")
}

func shouldFinalizeImplementationAfterMaxIterations(req RunnerRequest, traces []RuntimeToolTrace) bool {
	if req.Stage != StageImplementation {
		return false
	}
	evidence := implementationEvidenceFromTraces(req.Root, traces)
	return evidence.HasMutation && evidence.HasPassingTestCommand
}
