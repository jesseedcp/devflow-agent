package demandflow

import (
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestInspectConsoleRequirementsIsRunReady(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-req", Title: "Console req", Description: "Draft requirements", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	console, err := InspectConsole(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectConsole returned error: %v", err)
	}
	if console.PrimaryAction.Kind != ConsoleActionAgentStage {
		t.Fatalf("PrimaryAction.Kind = %q, want %q", console.PrimaryAction.Kind, ConsoleActionAgentStage)
	}
	if console.PrimaryAction.Stage != StageRequirements {
		t.Fatalf("PrimaryAction.Stage = %q, want %q", console.PrimaryAction.Stage, StageRequirements)
	}
	if !console.PrimaryAction.Runnable {
		t.Fatalf("PrimaryAction should be runnable: %#v", console.PrimaryAction)
	}
	if console.RunReadyAction.Label != console.PrimaryAction.Label {
		t.Fatalf("RunReadyAction = %#v, want primary action", console.RunReadyAction)
	}
}

func TestInspectConsoleVerificationPassRequiresHumanConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-pass", Title: "Console pass", Description: "Confirm verification", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: fixedConsoleTime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	console, err := InspectConsole(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectConsole returned error: %v", err)
	}
	if console.PrimaryAction.Kind != ConsoleActionHumanConfirmation {
		t.Fatalf("PrimaryAction.Kind = %q, want human confirmation", console.PrimaryAction.Kind)
	}
	if console.PrimaryAction.Runnable {
		t.Fatalf("human confirmation must not be runnable: %#v", console.PrimaryAction)
	}
	if console.PrimaryAction.BlockReason == "" {
		t.Fatalf("human confirmation action needs block reason: %#v", console.PrimaryAction)
	}
	if console.RunReadyAction.Kind != ConsoleActionNone {
		t.Fatalf("RunReadyAction = %#v, want none", console.RunReadyAction)
	}
}

func TestInspectConsoleMemoryPendingIsManual(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-memory", Title: "Console memory", Description: "Review memory", Source: "test", State: string(workflow.Completed)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.MemoryCandidatesFile, "# Memory Candidates: Console memory\n\n## 稳定知识候选\n\n- Reuse operator loop\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	console, err := InspectConsole(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectConsole returned error: %v", err)
	}
	if console.PrimaryAction.Kind != ConsoleActionMemoryReview {
		t.Fatalf("PrimaryAction.Kind = %q, want memory review", console.PrimaryAction.Kind)
	}
	if console.PrimaryAction.Runnable {
		t.Fatalf("memory review must not be runnable: %#v", console.PrimaryAction)
	}
	if console.RunReadyAction.Kind != ConsoleActionNone {
		t.Fatalf("RunReadyAction = %#v, want none", console.RunReadyAction)
	}
}

func TestListConsolePreservesWorkspacePriority(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	for _, demand := range []artifacts.Demand{
		{ID: "z-complete", Title: "Z complete", Description: "Done", Source: "test", State: string(workflow.Completed)},
		{ID: "a-failed", Title: "A failed", Description: "Failed", Source: "test", State: string(workflow.FailedQualityGate)},
		{ID: "b-verify", Title: "B verify", Description: "Verify", Source: "test", State: string(workflow.Verification)},
	} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
		}
	}

	summaries, err := ListConsole(root)
	if err != nil {
		t.Fatalf("ListConsole returned error: %v", err)
	}
	got := []string{summaries[0].Workspace.Demand.ID, summaries[1].Workspace.Demand.ID, summaries[2].Workspace.Demand.ID}
	want := []string{"a-failed", "b-verify", "z-complete"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %#v, want %#v", got, want)
		}
	}
}

func TestBuildConsoleActionClassifiesKnownCommands(t *testing.T) {
	summary := WorkspaceSummary{Demand: artifacts.Demand{ID: "known"}, State: workflow.Implementation}
	cases := []struct {
		name     string
		action   NextAction
		kind     ConsoleActionKind
		stage    Stage
		runnable bool
	}{
		{
			name:     "implementation stage",
			action:   NextAction{Label: "Run implementation", Command: `devflow run --demand known --stage implementation --permission-mode acceptEdits --quality-command "go test ./..."`},
			kind:     ConsoleActionAgentStage,
			stage:    StageImplementation,
			runnable: true,
		},
		{
			name:     "human confirmation",
			action:   NextAction{Label: "Confirm plan", Command: "devflow confirm --demand known --stage plan --by <name> --summary <summary>"},
			kind:     ConsoleActionHumanConfirmation,
			runnable: false,
		},
		{
			name:     "memory promote",
			action:   NextAction{Label: "Promote memory candidate", Command: "devflow memory promote --demand known --candidate <index> --by <name>"},
			kind:     ConsoleActionMemoryDecision,
			runnable: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildConsoleAction(summary, tc.action)
			if got.Kind != tc.kind || got.Stage != tc.stage || got.Runnable != tc.runnable {
				t.Fatalf("BuildConsoleAction = %#v, want kind=%s stage=%s runnable=%v", got, tc.kind, tc.stage, tc.runnable)
			}
		})
	}
}

func fixedConsoleTime() time.Time {
	return time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
}
