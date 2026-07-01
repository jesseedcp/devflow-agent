package cli

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func renderWorkbenchSnapshot(opts workbenchOptions) (string, error) {
	summaries, err := demandflow.ListConsole(opts.root)
	if err != nil {
		return "", err
	}
	selected := 0
	if opts.demandID != "" {
		selected = -1
		for index, summary := range summaries {
			if summary.Workspace.Demand.ID == opts.demandID {
				selected = index
				break
			}
		}
		if selected == -1 {
			return "", fmt.Errorf("demand %q not found", opts.demandID)
		}
	}

	var builder strings.Builder
	builder.WriteString("Devflow Workbench Snapshot\n\n")
	if len(summaries) == 0 {
		builder.WriteString("No demands found\n")
		return builder.String(), nil
	}
	for index, summary := range summaries {
		cursor := " "
		if index == selected {
			cursor = ">"
		}
		fmt.Fprintf(&builder, "%s %-24s %-22s %s\n", cursor, summary.Workspace.Demand.ID, summary.Workspace.State, summary.Workspace.Attention)
	}

	detail := summaries[selected]
	fmt.Fprintln(&builder, "\nSummary")
	fmt.Fprintf(&builder, "State: %s\n", detail.Workspace.State)
	fmt.Fprintf(&builder, "Attention: %s\n", detail.Workspace.Attention)
	fmt.Fprintln(&builder, "Evidence:")
	fmt.Fprintf(&builder, "  %-14s pass=%d fail=%d blocked=%d\n", "manual", detail.Workspace.Evidence.Pass, detail.Workspace.Evidence.Fail, detail.Workspace.Evidence.Blocked)
	ci := humanStatus(detail.Workspace.CIGate.Status)
	if detail.Workspace.CIGate.Repo != "" && detail.Workspace.CIGate.PR != "" {
		ci = detail.Workspace.CIGate.Repo + "#" + detail.Workspace.CIGate.PR + " " + ci
	}
	fmt.Fprintf(&builder, "  %-14s %s\n", "ci", ci)
	fmt.Fprintln(&builder, "Quality:")
	evaluation, err := demandflow.EvaluateDemand(opts.root, detail.Workspace.Demand.ID)
	if err != nil {
		fmt.Fprintf(&builder, "  unavailable: %v\n", err)
	} else {
		for _, stage := range evaluation.Stages {
			fmt.Fprintf(&builder, "  %-14s %s", stage.Stage, stage.Status)
			if stage.Blockers > 0 || stage.Warnings > 0 {
				fmt.Fprintf(&builder, " blockers=%d warnings=%d", stage.Blockers, stage.Warnings)
			}
			fmt.Fprintln(&builder)
			if stage.Stage == demandflow.StageRequirements {
				renderRequirementQualityChecks(&builder, stage, "    ")
			}
		}
	}
	fmt.Fprintln(&builder, "Next:")
	renderWorkbenchAction(&builder, detail.PrimaryAction)
	fmt.Fprintln(&builder, "Run-ready:")
	if detail.RunReadyAction.Runnable {
		renderWorkbenchAction(&builder, detail.RunReadyAction)
	} else {
		fmt.Fprintf(&builder, "  %s\n", detail.RunReadyAction.BlockReason)
	}
	return builder.String(), nil
}
