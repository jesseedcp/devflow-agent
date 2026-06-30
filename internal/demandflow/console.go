package demandflow

import (
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ConsoleActionKind string

const (
	ConsoleActionNone              ConsoleActionKind = "none"
	ConsoleActionAgentStage        ConsoleActionKind = "agent_stage"
	ConsoleActionHumanConfirmation ConsoleActionKind = "human_confirmation"
	ConsoleActionMemoryReview      ConsoleActionKind = "memory_review"
	ConsoleActionMemoryDecision    ConsoleActionKind = "memory_decision"
	ConsoleActionMRReview          ConsoleActionKind = "mr_review"
	ConsoleActionManual            ConsoleActionKind = "manual"
)

type ConsoleSummary struct {
	Workspace      WorkspaceSummary
	PrimaryAction  ConsoleAction
	RunReadyAction ConsoleAction
}

type ConsoleAction struct {
	Label       string
	Command     string
	Reason      string
	Kind        ConsoleActionKind
	Stage       Stage
	Runnable    bool
	BlockReason string
}

func InspectConsole(root, demandID string) (ConsoleSummary, error) {
	workspace, err := InspectWorkspace(root, demandID)
	if err != nil {
		return ConsoleSummary{}, err
	}
	return buildConsoleSummary(workspace), nil
}

func ListConsole(root string) ([]ConsoleSummary, error) {
	workspaces, err := ListWorkspaces(root)
	if err != nil {
		return nil, err
	}
	out := make([]ConsoleSummary, 0, len(workspaces))
	for _, workspace := range workspaces {
		out = append(out, buildConsoleSummary(workspace))
	}
	return out, nil
}

func buildConsoleSummary(workspace WorkspaceSummary) ConsoleSummary {
	primary := ConsoleAction{Kind: ConsoleActionNone, BlockReason: "no recommended action"}
	if len(workspace.Actions) > 0 {
		primary = BuildConsoleAction(workspace, workspace.Actions[0])
	}
	runReady := ConsoleAction{Kind: ConsoleActionNone, BlockReason: "no safe runner action"}
	if primary.Runnable {
		runReady = primary
	}
	return ConsoleSummary{
		Workspace:      workspace,
		PrimaryAction:  primary,
		RunReadyAction: runReady,
	}
}

func BuildConsoleAction(summary WorkspaceSummary, action NextAction) ConsoleAction {
	out := ConsoleAction{
		Label:       action.Label,
		Command:     action.Command,
		Reason:      action.Reason,
		Kind:        ConsoleActionManual,
		BlockReason: "manual action",
	}
	command := strings.TrimSpace(action.Command)
	if command == "" {
		out.Kind = ConsoleActionNone
		out.BlockReason = "no command available"
		if summary.State == workflow.Completed {
			out.BlockReason = "demand is complete"
		}
		return out
	}
	if strings.HasPrefix(command, "devflow confirm ") {
		out.Kind = ConsoleActionHumanConfirmation
		out.BlockReason = "human confirmation is required"
		return out
	}
	if strings.HasPrefix(command, "devflow memory list ") {
		out.Kind = ConsoleActionMemoryReview
		out.BlockReason = "memory review is a manual gate"
		return out
	}
	if strings.HasPrefix(command, "devflow memory promote ") || strings.HasPrefix(command, "devflow memory reject ") {
		out.Kind = ConsoleActionMemoryDecision
		out.BlockReason = "memory decisions require a human"
		return out
	}
	if strings.HasPrefix(command, "devflow run ") {
		stage, ok := stageFromDevflowRunCommand(command)
		if !ok {
			out.Kind = ConsoleActionManual
			out.BlockReason = "run command does not include a recognized stage"
			return out
		}
		out.Stage = stage
		if stage == StageMRReview {
			out.Kind = ConsoleActionMRReview
			out.BlockReason = "MR review requires explicit GitLab flags"
			return out
		}
		if isConsoleRunnableStage(stage) {
			out.Kind = ConsoleActionAgentStage
			out.Runnable = true
			out.BlockReason = ""
			return out
		}
	}
	out.Kind = ConsoleActionManual
	out.BlockReason = "manual action"
	if summary.State == workflow.Completed {
		out.Kind = ConsoleActionNone
		out.BlockReason = "demand is complete"
	}
	return out
}

func isConsoleRunnableStage(stage Stage) bool {
	switch stage {
	case StageRequirements, StagePlan, StageImplementation, StageVerification, StageCloseout:
		return true
	default:
		return false
	}
}

func stageFromDevflowRunCommand(command string) (Stage, bool) {
	fields := strings.Fields(command)
	for index := 0; index < len(fields)-1; index++ {
		if fields[index] != "--stage" {
			continue
		}
		stage, err := ParseStage(fields[index+1])
		return stage, err == nil
	}
	return "", false
}
