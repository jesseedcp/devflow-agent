package demandflow

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func promptContext() ContextSnapshot {
	return ContextSnapshot{
		Demand: artifacts.Demand{
			Title:       "Add coupon check",
			Description: "Only active members can claim coupons",
		},
		Artifacts: ArtifactSnapshot{
			Requirements: "# Requirements: Add coupon check\n\nbody",
			Plan:         "# Technical Plan: Add coupon check\n\nplan body",
			Progress:     "## 实现摘要\n\ndone",
			Verification: "# Verification: Add coupon check\n\nverified",
		},
		Memories: []MemoryHit{{DemandID: "prior-work", Snippet: "coupon knowledge"}},
	}
}

func TestBuildPromptRequirementsContract(t *testing.T) {
	t.Parallel()

	prompt, mode, err := BuildPrompt(StageRequirements, promptContext())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if mode != ToolModeReadOnly {
		t.Fatalf("mode = %q want read-only", mode)
	}
	if !strings.Contains(prompt, "Add coupon check") {
		t.Fatalf("prompt missing title: %q", prompt)
	}
	if !strings.Contains(prompt, "Only active members can claim coupons") {
		t.Fatalf("prompt missing description: %q", prompt)
	}
	if !strings.Contains(prompt, "requirements.md") {
		t.Fatalf("prompt missing requirements.md contract: %q", prompt)
	}
}

func TestBuildPromptPlanIncludesRequirements(t *testing.T) {
	t.Parallel()

	prompt, _, err := BuildPrompt(StagePlan, promptContext())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(prompt, "plan.md") {
		t.Fatalf("prompt missing plan.md contract: %q", prompt)
	}
	if !strings.Contains(prompt, "Requirements: Add coupon check") {
		t.Fatalf("plan prompt should include current requirements content: %q", prompt)
	}
}

func TestBuildPromptImplementationUsesEditAndShell(t *testing.T) {
	t.Parallel()

	_, mode, err := BuildPrompt(StageImplementation, promptContext())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if mode != ToolModeEditAndShell {
		t.Fatalf("mode = %q want edit-and-shell", mode)
	}
}

func TestBuildPromptCloseoutIncludesMemoryMarker(t *testing.T) {
	t.Parallel()

	prompt, _, err := BuildPrompt(StageCloseout, promptContext())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(prompt, "---DEVFLOW-MEMORY-CANDIDATES---") {
		t.Fatalf("closeout prompt missing memory marker: %q", prompt)
	}
	if !strings.Contains(prompt, "closeout.md") {
		t.Fatalf("prompt missing closeout.md contract: %q", prompt)
	}
}

func TestBuildPromptMRReviewUnsupported(t *testing.T) {
	t.Parallel()

	_, _, err := BuildPrompt(StageMRReview, promptContext())
	if err == nil {
		t.Fatalf("expected error for mr-review prompt, got nil")
	}
}
