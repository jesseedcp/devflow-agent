package demandflow

import (
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestDecideDriveStopForManualGates(t *testing.T) {
	cases := []struct {
		name   string
		action ConsoleAction
		want   DriveStopReason
	}{
		{"human confirmation", ConsoleAction{Kind: ConsoleActionHumanConfirmation, Label: "Confirm verification"}, DriveStopHumanConfirmation},
		{"memory review", ConsoleAction{Kind: ConsoleActionMemoryReview, Label: "Review memory"}, DriveStopMemoryGate},
		{"memory decision", ConsoleAction{Kind: ConsoleActionMemoryDecision, Label: "Promote memory"}, DriveStopMemoryGate},
		{"mr flags", ConsoleAction{Kind: ConsoleActionMRReview, Label: "Check MR review"}, DriveStopMRFlagsRequired},
		{"manual", ConsoleAction{Kind: ConsoleActionManual, Label: "Inspect manually"}, DriveStopManualAction},
		{"none completed", ConsoleAction{Kind: ConsoleActionNone, Label: "No action"}, DriveStopComplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := ConsoleSummary{Workspace: WorkspaceSummary{Demand: artifacts.Demand{ID: "drive"}, State: workflow.Completed}, PrimaryAction: tc.action}
			got := DecideDriveStop(summary, 0, 5)
			if got.Reason != tc.want {
				t.Fatalf("Reason = %s, want %s", got.Reason, tc.want)
			}
		})
	}
}

func TestDecideDriveStopAllowsRunnableAction(t *testing.T) {
	summary := ConsoleSummary{
		Workspace:     WorkspaceSummary{Demand: artifacts.Demand{ID: "drive"}, State: workflow.Created},
		PrimaryAction: ConsoleAction{Kind: ConsoleActionAgentStage, Stage: StageRequirements, Runnable: true, Label: "Draft requirements"},
	}
	got := DecideDriveStop(summary, 0, 5)
	if got.ShouldStop {
		t.Fatalf("ShouldStop = true, want false: %#v", got)
	}
}

func TestDecideDriveStopMaxSteps(t *testing.T) {
	summary := ConsoleSummary{
		Workspace:     WorkspaceSummary{Demand: artifacts.Demand{ID: "drive"}, State: workflow.Implementation},
		PrimaryAction: ConsoleAction{Kind: ConsoleActionAgentStage, Stage: StageImplementation, Runnable: true, Label: "Run implementation"},
	}
	got := DecideDriveStop(summary, 5, 5)
	if !got.ShouldStop || got.Reason != DriveStopMaxStepsReached {
		t.Fatalf("decision = %#v, want max steps stop", got)
	}
}
