package demandflow

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/runtime/agent"
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
)

func TestRuntimeRegistryContainsCoreFileTools(t *testing.T) {
	t.Parallel()

	registry := runtimeRegistry("anthropic")
	for _, name := range []string{"ReadFile", "WriteFile", "EditFile", "Bash", "Glob", "Grep", "ToolSearch"} {
		if registry.Get(name) == nil {
			t.Fatalf("runtime registry missing tool %s", name)
		}
	}
}

func TestPermissionModeForReadOnlyStagesMapsToPlan(t *testing.T) {
	t.Parallel()

	for _, stage := range []Stage{StageRequirements, StagePlan, StageVerification, StageCloseout} {
		mode, err := permissionModeFor(RunnerRequest{Stage: stage}, permissions.ModeBypass)
		if err != nil {
			t.Fatalf("stage %s: %v", stage, err)
		}
		if mode != permissions.ModePlan {
			t.Fatalf("stage %s mode = %q want plan", stage, mode)
		}
	}
}

func TestPermissionModeForImplementationRequiresExplicitMode(t *testing.T) {
	t.Parallel()

	if _, err := permissionModeFor(RunnerRequest{Stage: StageImplementation}, ""); err == nil {
		t.Fatalf("expected error for implementation without explicit permission mode")
	}

	mode, err := permissionModeFor(RunnerRequest{Stage: StageImplementation}, permissions.ModeAcceptEdits)
	if err != nil {
		t.Fatalf("acceptEdits: %v", err)
	}
	if mode != permissions.ModeAcceptEdits {
		t.Fatalf("mode = %q want acceptEdits", mode)
	}

	mode, err = permissionModeFor(RunnerRequest{Stage: StageImplementation}, permissions.ModeBypass)
	if err != nil {
		t.Fatalf("bypass: %v", err)
	}
	if mode != permissions.ModeBypass {
		t.Fatalf("mode = %q want bypassPermissions", mode)
	}
}

func TestPermissionModeForUnsupportedStageErrors(t *testing.T) {
	t.Parallel()

	if _, err := permissionModeFor(RunnerRequest{Stage: StageMRReview}, permissions.ModeBypass); err == nil {
		t.Fatalf("expected error for mr-review stage")
	}
}

func TestRuntimeEmptyOutputErrorIncludesStageAndIterations(t *testing.T) {
	t.Parallel()

	err := runtimeEmptyOutputError(StagePlan, "ark-code-latest", 20)
	if err == nil {
		t.Fatal("expected error")
	}
	message := err.Error()
	for _, want := range []string{"runtime runner produced no artifact text", "plan", "ark-code-latest", "20"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
}

func TestRuntimePermissionResponseAllowsPlanReadTools(t *testing.T) {
	req := RunnerRequest{Stage: StagePlan}

	for _, toolName := range []string{"ReadFile", "Glob", "Grep", "ToolSearch"} {
		resp := runtimePermissionResponse(req, permissions.ModePlan, agent.PermissionRequestEvent{
			ToolName: toolName,
			Desc:     "read-only planning lookup",
		})
		if resp != agent.PermAllow {
			t.Fatalf("runtimePermissionResponse(%s) = %v, want allow", toolName, resp)
		}
	}
}

func TestRuntimePermissionResponseDeniesPlanMutationTools(t *testing.T) {
	req := RunnerRequest{Stage: StagePlan}

	for _, toolName := range []string{"WriteFile", "EditFile", "Bash"} {
		resp := runtimePermissionResponse(req, permissions.ModePlan, agent.PermissionRequestEvent{
			ToolName: toolName,
			Desc:     "mutation should not be allowed in plan stage",
		})
		if resp != agent.PermDeny {
			t.Fatalf("runtimePermissionResponse(%s) = %v, want deny", toolName, resp)
		}
	}
}

func TestRuntimePermissionResponseAllowsImplementationBypassRequests(t *testing.T) {
	req := RunnerRequest{Stage: StageImplementation}

	for _, toolName := range []string{"ReadFile", "WriteFile", "EditFile", "Bash"} {
		resp := runtimePermissionResponse(req, permissions.ModeBypass, agent.PermissionRequestEvent{
			ToolName: toolName,
			Desc:     "bypass mode should answer tool asks affirmatively after checker produced Ask",
		})
		if resp != agent.PermAllow {
			t.Fatalf("runtimePermissionResponse(%s) = %v, want allow", toolName, resp)
		}
	}
}

func TestRuntimeAgentErrorIncludesToolSummary(t *testing.T) {
	err := runtimeAgentError(StageImplementation, "glm-5.2", 20, []string{"ReadFile", "Grep", "ReadFile"}, "Agent reached maximum iterations (20)")
	if err == nil {
		t.Fatal("runtimeAgentError returned nil")
	}
	msg := err.Error()
	for _, want := range []string{"implementation", "glm-5.2", "maximum iterations", "ReadFile", "Grep", "tool calls=3"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want %q", msg, want)
		}
	}
}

func TestCollectRuntimeTraceUseRecordsDescriptions(t *testing.T) {
	descs := map[string]string{}

	collectRuntimeTraceUse(descs, agent.ToolUseEvent{
		ToolID:   "edit1",
		ToolName: "EditFile",
		Args: map[string]any{
			"file_path": "internal/weather/service.go",
		},
	})
	collectRuntimeTraceUse(descs, agent.ToolUseEvent{
		ToolID:   "bash1",
		ToolName: "Bash",
		Args: map[string]any{
			"command": "go test ./...",
		},
	})

	if descs["edit1"] != "internal/weather/service.go" {
		t.Fatalf("edit desc = %q", descs["edit1"])
	}
	if descs["bash1"] != "go test ./..." {
		t.Fatalf("bash desc = %q", descs["bash1"])
	}
}

func TestCollectRuntimeTraceResultBuildsTrace(t *testing.T) {
	descs := map[string]string{"bash1": "go test ./..."}
	trace := collectRuntimeTraceResult(descs, agent.ToolResultEvent{
		ToolID:   "bash1",
		ToolName: "Bash",
		Output:   "ok",
		IsError:  false,
	})

	if trace.ToolID != "bash1" || trace.ToolName != "Bash" || trace.Desc != "go test ./..." || trace.Output != "ok" {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestMaybeFinalizeRuntimeErrorReturnsProgressForSafeImplementationMaxIterations(t *testing.T) {
	req := RunnerRequest{Stage: StageImplementation}
	traces := []RuntimeToolTrace{
		{ToolName: "EditFile", Desc: "tools.go", Output: "Successfully edited tools.go"},
		{ToolName: "Bash", Desc: "go test ./...", Output: "ok", IsError: false},
	}

	body, summary, ok := maybeFinalizeRuntimeError(req, "glm-5.2", 20, traces, "Agent reached maximum iterations (20)")
	if !ok {
		t.Fatal("maybeFinalizeRuntimeError ok = false, want true")
	}
	if !strings.Contains(body, "deterministic runtime finalizer") {
		t.Fatalf("body missing finalizer marker:\n%s", body)
	}
	if summary.CompletionMode != RuntimeCompletionDeterministicFinalizer {
		t.Fatalf("CompletionMode = %s", summary.CompletionMode)
	}
	if !summary.MaxIterationsHit {
		t.Fatal("MaxIterationsHit = false")
	}
}

func TestMaybeFinalizeRuntimeErrorRejectsNonImplementationMaxIterations(t *testing.T) {
	req := RunnerRequest{Stage: StagePlan}
	_, _, ok := maybeFinalizeRuntimeError(req, "glm-5.2", 20, []RuntimeToolTrace{
		{ToolName: "EditFile", Desc: "plan.md", Output: "edited"},
		{ToolName: "Bash", Desc: "go test ./...", Output: "ok"},
	}, "Agent reached maximum iterations (20)")
	if ok {
		t.Fatal("plan stage should not use implementation finalizer")
	}
}
