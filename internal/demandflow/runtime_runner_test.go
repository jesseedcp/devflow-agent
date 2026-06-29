package demandflow

import (
	"testing"

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
