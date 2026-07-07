package demandflow

import (
	"strings"
	"testing"
	"time"
)

func TestRuntimeSummaryCountsToolTraces(t *testing.T) {
	traces := []RuntimeToolTrace{
		{ToolName: "ReadFile", Output: "ok", Elapsed: time.Millisecond},
		{ToolName: "EditFile", Desc: "internal/weather/service.go", Output: "Successfully edited internal/weather/service.go"},
		{ToolName: "Bash", Desc: "go test ./...", Output: "ok", Elapsed: 2 * time.Millisecond},
	}

	summary := summarizeRuntimeTraces(StageImplementation, "glm-5.2", RuntimeCompletionModelText, false, traces, []string{"internal/weather/service.go"})

	if summary.Stage != StageImplementation {
		t.Fatalf("Stage = %s, want implementation", summary.Stage)
	}
	if summary.Model != "glm-5.2" {
		t.Fatalf("Model = %q, want glm-5.2", summary.Model)
	}
	if summary.ToolCalls != 3 {
		t.Fatalf("ToolCalls = %d, want 3", summary.ToolCalls)
	}
	if summary.EditCalls != 1 {
		t.Fatalf("EditCalls = %d, want 1", summary.EditCalls)
	}
	if summary.BashCalls != 1 {
		t.Fatalf("BashCalls = %d, want 1", summary.BashCalls)
	}
	if summary.ChangedFiles[0] != "internal/weather/service.go" {
		t.Fatalf("ChangedFiles = %#v", summary.ChangedFiles)
	}
}

func TestRuntimeToolTraceRedactedExcerpt(t *testing.T) {
	trace := RuntimeToolTrace{
		ToolName: "Bash",
		Desc:     "curl -H Authorization: Bearer secret-token https://example.test?token=abc",
		Output:   `Authorization: Bearer secret-token {"password":"pw"}`,
	}

	got := trace.RedactedOutputExcerpt(96)
	for _, leaked := range []string{"secret-token", "token=abc", `"password":"pw"`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redacted excerpt leaked %q: %s", leaked, got)
		}
	}
}

func TestImplementationEvidenceDetectsMutationAndPassingTests(t *testing.T) {
	traces := []RuntimeToolTrace{
		{ToolName: "EditFile", Desc: "tools.go", Output: "Successfully edited tools.go"},
		{ToolName: "WriteFile", Desc: "main_test.go", Output: "Successfully wrote to main_test.go"},
		{ToolName: "Bash", Desc: "go test ./...", Output: "ok   weather-agent 0.123s", IsError: false},
	}

	evidence := implementationEvidenceFromTraces("", traces)

	if !evidence.HasMutation {
		t.Fatal("HasMutation = false, want true")
	}
	if !evidence.HasPassingTestCommand {
		t.Fatal("HasPassingTestCommand = false, want true")
	}
	if len(evidence.ChangedFiles) != 2 {
		t.Fatalf("ChangedFiles = %#v, want two files", evidence.ChangedFiles)
	}
	if evidence.TestCommands[0] != "go test ./..." {
		t.Fatalf("TestCommands = %#v", evidence.TestCommands)
	}
}

func TestShouldFinalizeImplementationAfterMaxIterationsRequiresMutationAndPassingTest(t *testing.T) {
	req := RunnerRequest{Stage: StageImplementation}

	ok := shouldFinalizeImplementationAfterMaxIterations(req, []RuntimeToolTrace{
		{ToolName: "EditFile", Desc: "tools.go", Output: "Successfully edited tools.go"},
		{ToolName: "Bash", Desc: "go test ./...", Output: "ok", IsError: false},
	})
	if !ok {
		t.Fatal("shouldFinalizeImplementationAfterMaxIterations = false, want true")
	}

	noMutation := shouldFinalizeImplementationAfterMaxIterations(req, []RuntimeToolTrace{
		{ToolName: "Bash", Desc: "go test ./...", Output: "ok", IsError: false},
	})
	if noMutation {
		t.Fatal("finalized without mutation")
	}

	noPassingTest := shouldFinalizeImplementationAfterMaxIterations(req, []RuntimeToolTrace{
		{ToolName: "EditFile", Desc: "tools.go", Output: "Successfully edited tools.go"},
		{ToolName: "Bash", Desc: "go test ./...", Output: "FAIL", IsError: true},
	})
	if noPassingTest {
		t.Fatal("finalized without passing test command")
	}
}
