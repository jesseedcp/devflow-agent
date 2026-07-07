package demandflow

import (
	"fmt"
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

func renderImplementationRuntimeFinalizer(model string, maxIterations int, traces []RuntimeToolTrace, changedFiles []string) string {
	evidence := implementationEvidenceFromTraces("", traces)
	files := uniqueStrings(append(changedFiles, evidence.ChangedFiles...))
	commands := evidence.TestCommands

	var b strings.Builder
	b.WriteString("## 实现摘要\n\n")
	b.WriteString("- Devflow deterministic runtime finalizer generated this progress summary because the implementation runtime reached the max-iteration limit after useful tool work.\n")
	b.WriteString(fmt.Sprintf("- Model: %s\n", model))
	b.WriteString(fmt.Sprintf("- Max iterations: %d\n", maxIterations))
	b.WriteString(fmt.Sprintf("- Tool calls observed: %d\n", len(traces)))
	b.WriteString("- Existing quality gates and implementation review must still decide whether the work is acceptable.\n\n")

	b.WriteString("## 代码改动\n\n")
	if len(files) == 0 {
		b.WriteString("- No changed files were detected by tool traces or git diff.\n")
	} else {
		for _, file := range files {
			b.WriteString("- ")
			b.WriteString(file)
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')

	b.WriteString("## 测试与验证\n\n")
	if len(commands) == 0 {
		b.WriteString("- No passing test-like command was detected in runtime tool output.\n")
	} else {
		for _, command := range commands {
			b.WriteString("- PASS: `")
			b.WriteString(evidenceadapter.Redact(command))
			b.WriteString("`\n")
		}
	}
	for _, excerpt := range evidence.PassingCommandExcerpts {
		if strings.TrimSpace(excerpt) == "" {
			continue
		}
		b.WriteString("\n```text\n")
		b.WriteString(excerpt)
		b.WriteString("\n```\n")
		break
	}
	b.WriteByte('\n')

	b.WriteString("## 遗留问题\n\n")
	b.WriteString("- Runtime reached max iterations before the model returned normal final artifact text; this summary is deterministic and should be reviewed with implementation-review.\n")
	return b.String()
}
