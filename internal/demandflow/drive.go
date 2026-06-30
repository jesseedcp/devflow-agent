package demandflow

import "github.com/jesseedcp/devflow-agent/internal/workflow"

type DriveStopReason string

const (
	DriveStopNone              DriveStopReason = ""
	DriveStopHumanConfirmation DriveStopReason = "human_confirmation"
	DriveStopMemoryGate        DriveStopReason = "memory_gate"
	DriveStopMRFlagsRequired   DriveStopReason = "mr_flags_required"
	DriveStopRunnerFailed      DriveStopReason = "runner_failed"
	DriveStopMaxStepsReached   DriveStopReason = "max_steps_reached"
	DriveStopComplete          DriveStopReason = "complete"
	DriveStopManualAction      DriveStopReason = "manual_action"
)

type DriveStep struct {
	Number        int
	Action        ConsoleAction
	PreviousState workflow.State
	CurrentState  workflow.State
	Message       string
}

type DriveReport struct {
	DemandID   string
	Steps      []DriveStep
	StopReason DriveStopReason
	NextAction ConsoleAction
}

type DriveDecision struct {
	ShouldStop bool
	Reason     DriveStopReason
	Action     ConsoleAction
	Message    string
}

func DecideDriveStop(summary ConsoleSummary, completedSteps, maxSteps int) DriveDecision {
	action := summary.PrimaryAction
	if maxSteps > 0 && completedSteps >= maxSteps {
		return DriveDecision{ShouldStop: true, Reason: DriveStopMaxStepsReached, Action: action, Message: "max steps reached"}
	}

	switch action.Kind {
	case ConsoleActionAgentStage:
		if action.Runnable {
			return DriveDecision{Action: action}
		}
		return DriveDecision{ShouldStop: true, Reason: DriveStopManualAction, Action: action, Message: action.BlockReason}
	case ConsoleActionHumanConfirmation:
		return DriveDecision{ShouldStop: true, Reason: DriveStopHumanConfirmation, Action: action, Message: action.BlockReason}
	case ConsoleActionMemoryReview, ConsoleActionMemoryDecision:
		return DriveDecision{ShouldStop: true, Reason: DriveStopMemoryGate, Action: action, Message: action.BlockReason}
	case ConsoleActionMRReview:
		return DriveDecision{ShouldStop: true, Reason: DriveStopMRFlagsRequired, Action: action, Message: action.BlockReason}
	case ConsoleActionNone:
		reason := DriveStopManualAction
		if summary.Workspace.State == workflow.Completed || action.Label == "No action" {
			reason = DriveStopComplete
		}
		return DriveDecision{ShouldStop: true, Reason: reason, Action: action, Message: action.BlockReason}
	default:
		return DriveDecision{ShouldStop: true, Reason: DriveStopManualAction, Action: action, Message: action.BlockReason}
	}
}
